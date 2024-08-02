package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"io"
	"strconv"
	"strings"
	"text/template"
	"time"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"sync"
)

var (
	runners int
	runnersMu sync.Mutex
	cmdQ []Cmd
	tested bool
	cmdCallLeftCountMu sync.Mutex
	cmdCallLeftCount map[string]int
	totalActiveCalls int
	cmdActiveCalls int
	maxCalls int
	ports Ports
	portsMu sync.Mutex
	count int
)

func cmdIsCallsLeft(uuid string) (bool){
	cmdCallLeftCountMu.Lock()
	defer cmdCallLeftCountMu.Unlock()
	i, found := cmdCallLeftCount[uuid]
	if !found {
		return false
	}
	if i > 0 {
		return true
	}
	return false
}

func cmdIncCallLeft(uuid string, x int) (int){
	cmdCallLeftCountMu.Lock()
	defer cmdCallLeftCountMu.Unlock()
	cmdCallLeftCount[uuid] = x
	totalActiveCalls = totalActiveCalls + x
	return totalActiveCalls
}

func cmdDecCallLeft(uuid string, x int) (int){
	cmdCallLeftCountMu.Lock()
	defer cmdCallLeftCountMu.Unlock()
	i, found := cmdCallLeftCount[uuid]
	if !found {
		return -1
	}
	i = i - x
	totalActiveCalls = totalActiveCalls - x
	if i < 1 {
		delete(cmdCallLeftCount, uuid)
	} else {
		cmdCallLeftCount[uuid] = i
	}
	return i
}

func runnersActive() bool {
	runnersMu.Lock()
	defer runnersMu.Unlock()
	if runners > 0 {
		return true
	}
	return false
}

func runnersCount() int {
	runnersMu.Lock()
	defer runnersMu.Unlock()
	return runners
}

func runnersInc() {
	runnersMu.Lock()
	runners++
	runnersMu.Unlock()
}

func runnersDec() {
	runnersMu.Lock()
	runners--
	runnersMu.Unlock()
}

type Call struct {
	Idx int
	Ruri string `json:"destination"`
	From string `json:"from"`
	Count int `json:"count"`
	Username string `json:"username"`
	Password string `json:"password"`
	Duration int `json:"duration"`
	Uuid string `json:"uuid"`
	InboundSrcIp string `json:"inbound_source_ip"`
	Allow string `json:"allow"`
	EarlyRecord int
	ExpectedCauseCode int16
}

type CallParams struct {
	Ruri string
	From string
	Repeat int
	Username string
	Password string
	Duration int
	EarlyRecord int
	PortRtp uint16
	PortSip uint16
	Idx int
	Uuid string
	IpAddr string
	BoundAddr string
	ExpectedCauseCode int16
}

type Cmd struct {
	CallsIn []Call
	CallsOut []Call
	Calls []Call   `json:"calls"`
	Uuid string
	Context string `json:"context"`
	Type string    `json:"type"`
	Cps int        `json:"cps"`
	CallCount int
}

type RtpTransfer struct {
	JitterAvg float32 `json:"jitter_avg"`
	JitterMax float32 `json:"jitter_max"`
	Pkt       int32   `json:"pkt"`
	Kbytes    int32   `json:"kbytes"`
	Loss      int32   `json:"loss"`
	Mos       float32 `json:"mos_lq"`
}

type RtpStats struct {
	Rtt          int         `json:"rtt"`
	RemoteSocket string      `json:"remote_rtp_socket"`
	CodecName    string      `json:"codec_name"`
	CodecRate    string      `json:"codec_rate"`
	Tx           RtpTransfer `json:"Tx"`
	Rx           RtpTransfer `json:"Rx"`
}

type SipLatency struct {
	Invite100Ms int32 `json: "invite100Ms"`
	Invite18xMs int32 `json: "invite18xMs"`
	Invite200Ms int32 `json: "invite200Ms"`
}

type CallInfo struct {
	LocalUri      string `json:"local_uri"`
	RemoteUri     string `json:"remote_uri"`
	LocalContact  string `json:"local_contact"`
	RemoteContact string `json:"remote_contact"`
}

type TestReport struct {
	Label            string     `json:"label"`
	Start            string     `json:"start"`
	End              string     `json:"end"`
	Action           string     `json:"action"`
	From             string     `json:"from"`
	To               string     `json:"to"`
	Result           string     `json:"result"`
	ExpectedCauseCode int16     `json:"expected_cause_code"`
	CauseCode        int32      `json:"cause_code"`
	Reason           string     `json:"reason"`
	ToneDetected     int32      `json:"tone_detected"`
	CallId           string     `json:"callid"`
	Transport        string     `json:"transport"`
	PeerSocket       string     `json:"peer_socket"`
	Duration         int32      `json:"duration"`
	ExpectedDuration int32      `json:"expected_duration"`
	MaxDuration      int32      `json:"max_duration"`
	HangupDuration   int32      `json:"hangup_duration"`
	CallInfo         CallInfo   `json:"call_info"`
	SipLatency       SipLatency `json:"sip_latency"`
	RtpStats         []RtpStats `json:"rtp_stats"`
}

type ReportRtpPkt struct {
	Pkt       int32   `json:"pkt"`
	Kbytes    int32   `json:"kbytes"`
	Lost      int32   `json:"lost"`
	JitterMax float32 `json:"jitter_max"`
}

type ReportRtp struct {
	RttAvg    int32 `json:"rtt_avg"`
	Tx ReportRtpPkt `json:"tx"`
	Rx ReportRtpPkt `json:"rx"`
}

type Stat struct {
	Min int32       `json:"min_ms"`
	Max int32       `json:"max_ms"`
	Average float32 `json:"avg_ms"`
	Stdev float32   `json:"std_ms"`   // last standard deviation
	m2 float64 // sum of squares, used for recursive variance calculation
	Count int32     `json:"count"`
}

type ReportSip struct {
	Invite100 Stat `json:"invite100"`
	Invite18x Stat `json:"invite18x"`
	Invite200 Stat `json:"invite200"`
}

type Report struct {
	Uuid        string  `json:"uuid"`
	Calls       int32   `json:"calls"`
	Duration    int32   `json:"duration"`
	AvgDuration float32 `json:"avg_duration"`
	Failed      int32     `json:"failed"`
	Connected   int32     `json:"connected"`
	Reachable   int32     `json:"reachable"`
	Sip         ReportSip `json:"sip"`
	Rtp         ReportRtp `json:"rtp"`
}

// Compile templates on start of the application
var templates = template.Must(template.ParseFiles("public/cmd.html"))

// Display the named template
func display(w http.ResponseWriter, page string) {
	type Vars struct {
		LocalIp string
		VpServer string
	}
	data := Vars{os.Getenv("LOCAL_IP"), os.Getenv("VP_SERVER_IP")} 
	err := templates.ExecuteTemplate(w, page+".html", data)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func cmdDockerExec(uuid string, idx int, callCount int, portSip uint16, portRtp uint16, ipAddr string, boundAddr string) (error) {
	defer runnersDec()
	sport := fmt.Sprintf("%d", portSip)
	rport := fmt.Sprintf("%d", portRtp)
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		cmdDecCallLeft(uuid, callCount)
		return err
	}
	ctx := context.Background()
	fmt.Printf("client created... [%s]\n", sport)
	cli.NegotiateAPIVersion(ctx)

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		cmdDecCallLeft(uuid, callCount)
		return err
	}
	containerId := ""
	containerName := ""
	for _, ctr := range containers {
		if strings.Contains(ctr.Image, "hct_client") {
			containerId = ctr.ID
			containerName = ctr.Image
			fmt.Printf("hct_client container running %s %s\n", containerName, containerId)
			break
		}
	}
	if containerId == "" {
		err := errors.New("hct_client container not running\n")
		return err
	}

	xml_fn := fmt.Sprintf("/xml/hct/%s-%d.xml", uuid, idx)
	out_fn := fmt.Sprintf("/output/%s-%d.json", uuid, idx)
	log_fn := fmt.Sprintf("/output/%s-%d.log", uuid, idx)
	cmd := []string{"/git/voip_patrol/voip_patrol", "--udp",  "--rtp-port", rport,
                        "--port", sport,
                        "--conf", xml_fn,
                        "--output", out_fn,
                        "--log", log_fn,
                        "--ip-addr", ipAddr,
                        "--bound-addr", boundAddr,
                        "--log-level-file", os.Getenv("VP_LOG_LEVEL"),
                        "--log-level-console", os.Getenv("VP_LOG_LEVEL")}
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd: cmd,
	}

	response, err := cli.ContainerExecCreate(ctx, containerId, execConfig)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		return err
	}
	startConfig := types.ExecStartCheck{
		Detach: false,
		Tty:    false,
	}
	fmt.Printf("ContainerExecCreate [%s] cmd %s\n", containerId, cmd)
	runnersInc()
	err = cli.ContainerExecStart(ctx, response.ID, startConfig)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		runnersDec()
		return err
	}
	fmt.Printf("ContainerExecStart [%s]\n", containerId)
	execInspect, err := cli.ContainerExecInspect(ctx, response.ID)
	fmt.Printf("ContainerExecInspect >> pid[%d]running[%t]\n", execInspect.Pid, execInspect.Running)
	time.Sleep(10 * time.Second)
	for execInspect.Running {
		time.Sleep(1000 * time.Millisecond)
		execInspect, err = cli.ContainerExecInspect(ctx, response.ID)
		fmt.Printf("ContainerExecInspect >> pid[%d]running[%t]\n", execInspect.Pid, execInspect.Running)
	}
	x := cmdDecCallLeft(uuid, callCount)
	portsFreeSipPort(portSip)
	portsFreeRtpPort(portRtp)
	fmt.Printf("uuid[%s] idx[%d] calls completed[%d] left[%d] total_active_Calls[%d]\n", uuid, idx, callCount, x, totalActiveCalls)
	if x == 0 {
		report, err := resGetReport(uuid)
		if err != nil { 
			return err
		} else {
			rmqPublish(report, os.Getenv("RMQ_PUB_KEY_SUMMARY"))
			cleanUp(uuid)
		}
	}
	return nil
}

func createXmlFile(uuid string, idx int, xml string) (error) {
	// Create file
	dst, err := os.Create(fmt.Sprintf("/xml/hct/%s-%d.xml", uuid, idx))
	if err != nil {
		return err
	}
	defer dst.Close()
	if err := os.WriteFile(fmt.Sprintf("/xml/hct/%s-%d.xml", uuid, idx), []byte(xml), 0666); err != nil {
		return err
	}
	fmt.Printf("createXmlFile: %s\n",fmt.Sprintf("/xml/hct/%s-%d.xml", uuid, idx))
	return nil
}

type Ports struct {
	sip_start uint16
	sip_end uint16
	sip map[uint16]bool
	rtp_start uint16
	rtp_end uint16
	rtp map[uint16]bool
}

func portsInit(sip_start uint16, sip_end uint16, rtp_start uint16, rtp_end uint16) error {
	ports.sip_start = sip_start
	ports.sip_end = sip_end
	ports.sip = make(map[uint16]bool)
	ports.rtp = make(map[uint16]bool)
	if sip_end < sip_start {
		return fmt.Errorf("invalid sip port range %d !<= %d", sip_start, sip_end)
	}
	ports.rtp_start = rtp_start
	ports.rtp_end = rtp_end
	if rtp_end < rtp_start {
		return fmt.Errorf("invalid rtp port range %d !<= %d", rtp_start, rtp_end)
	}
	for i:=ports.sip_start; i<=ports.sip_end; i++ {
		ports.sip[i] = false
	}
	for i:=ports.rtp_start; i<=ports.rtp_end; i++ {
		ports.rtp[i] = false
	}
	return nil
}

func portsFreeRtpPort(p uint16) {
	portsMu.Lock()
	defer portsMu.Unlock()
	ports.rtp[p] = false
}

func portsGetRtpPort() (uint16) {
	portsMu.Lock()
	blockSize := uint16(200)
	defer portsMu.Unlock()
	for i:=ports.rtp_start; i<=ports.rtp_end; i=i+blockSize {
		if (ports.rtp[i] == false) {
			ports.rtp[i] = true
			fmt.Printf("portsGetRtpPort: %d\n", i)
			return i
		}
		i++
	}
	return 0
}

func portsFreeSipPort(p uint16) {
	portsMu.Lock()
	defer portsMu.Unlock()
	ports.sip[p] = false
}

func portsGetSipPort() (uint16) {
	portsMu.Lock()
	defer portsMu.Unlock()
	for i:=ports.sip_start; i<=ports.sip_end; i++ {
		if (ports.sip[i] == false) {
			ports.sip[i] = true
			fmt.Printf("portsGetSipPort: %d\n", i)
			return i
		}
		i++
	}
	return 0
}

func cmdCallCreateParams(cmd Cmd, c Call, idx int) (CallParams, error) {
	p := CallParams{c.Ruri, c.From, 0, c.Username, c.Password,
	                c.Duration, c.EarlyRecord, portsGetRtpPort(), portsGetSipPort(), idx, cmd.Uuid, "", "", c.ExpectedCauseCode}
	if cmd.Context == "customer" {
		p.IpAddr = os.Getenv("PUBLIC_IP_CUSTOMER");
		p.BoundAddr = os.Getenv("PRIVATE_IP_CUSTOMER");
	} else if cmd.Context == "provider" {
		p.IpAddr = os.Getenv("PUBLIC_IP_PROVIDER");
		p.BoundAddr = os.Getenv("PRIVATE_IP_PROVIDER");
	}
	return p, nil
}

func cmdMakeCalls(cmd Cmd) (error) {
	var CallsParams []CallParams
	for i, c := range cmd.CallsIn {
		repeat := c.Count
		fmt.Printf("cmdMakeCalls: proceeding with test, count[%d]\n", repeat)
		if repeat > 0 {
			for repeat > 0 {
				params, _ := cmdCallCreateParams(cmd, c, i)
				if repeat < 50 {
					params.Repeat = repeat-1
					repeat = 0
				} else {
					params.Repeat = 49
					repeat -= 50
				}
				CallsParams = append(CallsParams, params)
				go cmdExecCall(CallsParams)
				CallsParams = CallsParams[:0]
				time.Sleep(250 * time.Millisecond)
			}
		} else {
			params, _ := cmdCallCreateParams(cmd, c, i)
			CallsParams = append(CallsParams, params)
			if i >= 0 && i % 50 == 0 {
				go cmdExecCall(CallsParams)
				CallsParams = CallsParams[:0]
				time.Sleep(2100 * time.Millisecond)
			}
		}
	}
	return nil
}

const N2T_CODE = 800;

func cmdCreateCall(cmd *Cmd, cmdQ *[]Cmd,  context string, idx int) (error) {
	if cmd.Context == "" {
		cmd.Context = context
	}

	for i := range cmd.CallsIn {

		if cmd.Type == "n2t" { // n2t: network number tester
			cmd.CallsIn[i].EarlyRecord = 1
			cmd.CallsIn[i].Duration = 0
			cmd.CallsIn[i].Count = 1
			cmd.CallsIn[i].ExpectedCauseCode = N2T_CODE // This is a hack to identify the test type when looking at the result.
		}
		fmt.Printf("cmd[%s] %d\n", cmd.Type, cmd.CallsIn[i].EarlyRecord);
		if cmd.CallsIn[i].Allow != "" {
			host := os.Getenv("VP_SERVER_IP")+":"+os.Getenv("VP_SERVER_PORT")
			code, _ := AllowIp(host, cmd.CallsIn[i].Allow)
			if code != 200 {
				err := errors.New(fmt.Sprintf("allow UP failed with code : %d\n", code))
				return err
			}
		}
		if cmd.CallsIn[i].Ruri != "" {
			fmt.Printf("cmdCreateCall[%s]\n", cmd.CallsIn[i].Ruri)
		} else {
			err := errors.New("invalid command, empty request URI\n")
			return err
		}
		if cmd.CallsIn[i].From == "" {
			cmd.CallsIn[i].From = "hct_controller"
		}
		if cmd.CallsIn[i].ExpectedCauseCode == 0 {
			cmd.CallsIn[i].ExpectedCauseCode = 200
		}
		if cmd.CallsIn[i].Uuid == "" {
			cmd.CallsIn[i].Uuid = cmd.Uuid
		}
		cmd.CallsIn[i].Idx = idx
		cmd.CallsOut = append(cmd.CallsOut, cmd.CallsIn[i])
	}
	*cmdQ = append(*cmdQ, *cmd)
	return nil
}

func cmdCreate(s string, cmdQ *[]Cmd,  context string) (string, error) {
	cmd := new(Cmd)
	b := []byte(s)

	err := json.Unmarshal(b, cmd)
	if err != nil {
		fmt.Printf("invalid command [%s][%s]\n", cmd, err)
		return "", err
	}
	if cmd.Uuid == "" {
		cmd.Uuid = uuid.NewString()
	}
	count := 0
	for i := 0; i < len(cmd.Calls); i++ {
		if cmd.Calls[i].Count == 0 {
			count++
		} else {
			count = count + cmd.Calls[i].Count
		}
	}
	if count + totalActiveCalls > maxCalls {
		fmt.Printf("too many active calls, adding to queue %d > %d (max calls) \n", count, maxCalls)
	}
	if count > maxCalls {
		fmt.Printf("too many calls requested %d > %d (max calls)\n", count, maxCalls)
		err := errors.New("too many calls requested")
		return cmd.Uuid, err
	}
	cmdIncCallLeft(cmd.Uuid, count)
	fmt.Printf(">>>>> cmdCreate: calls[%d] count[%d] <<<<<\n", len(cmd.Calls), count)
	// create and queue each call
	i := 0
	for ; i < len(cmd.Calls); i++ {
		cmd.CallsIn = append(cmd.CallsIn, cmd.Calls[i])
	}
	err = cmdCreateCall(cmd, cmdQ, context, i)
	if err != nil {
		fmt.Printf("error creating call [%s][%s]\n", cmd, err)
		return cmd.Uuid, err
	}
	return cmd.Uuid, nil
}

// Compile templates on start of the application
var templates_ui = template.Must(template.ParseFiles("public/upload.html"))
// Display the named template
func display_ui(w http.ResponseWriter, page string, data interface{}) {
        templates_ui.ExecuteTemplate(w, page+".html", data)
}
func dockerExecSendFax(fn string) (error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	ctx := context.Background()
	fmt.Printf("docker client created...\n")
	cli.NegotiateAPIVersion(ctx)

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
	}
	containerId := ""
	containerName := ""
	for _, ctr := range containers {
		if strings.Contains(ctr.Image, "freeswitch") {
			containerId = ctr.ID
			containerName = ctr.Image
			fmt.Printf("freeswitch container running %s %s\n", containerName, containerId)
			break

		}
			fmt.Printf("container running %s %s\n", ctr.ID, ctr.Image)
	}
	if containerId == "" {
		err := errors.New("freeswitch container not running\n")
		return err
	}
	// cmd := []string{"echo", "fs_cli", fn}
	// cmd := []string{"gs", "-q", "-dNOPAUSE", "-sDEVICE=tiffg4", "-sOutputFile=/files/upload/"+fn+".tiff", "/files/upload/"+fn,"-c","quit"}
	cmd_gs := exec.Command("gs", "-q", "-dNOPAUSE", "-sDEVICE=tiffg4", "-sOutputFile=/files/upload/"+fn+".tiff", "/files/upload/"+fn,"-c","quit")
	output, err := cmd_gs.Output()
	if err != nil {
		fmt.Println("Error executing command:", err)
 		return err
	}
	fmt.Println(string(output))
	// gs -q -dNOPAUSE -sDEVICE=tiffg4 -sOutputFile=/files/upload/tx.tiff /files/upload/invoice.pdf -c quit
	cmd := []string{"/usr/local/freeswitch/bin/fs_cli","-x", "originate {absolute_codec_string='PCMU'}sofia/external/fax@15.222.241.45:5062 &txfax(/files/upload/"+fn+".tiff)"}

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd: cmd,
	}

	response, err := cli.ContainerExecCreate(ctx, containerId, execConfig)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		return err
	}
	startConfig := types.ExecStartCheck{
		Detach: false,
		Tty:    false,
	}
	fmt.Printf("ContainerExecCreate [%s] cmd %s\n", containerId, cmd)
	runnersInc()
	err = cli.ContainerExecStart(ctx, response.ID, startConfig)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		runnersDec()
		return err
	}
	fmt.Printf("ContainerExecStart [%s]\n", containerId)
	execInspect, err := cli.ContainerExecInspect(ctx, response.ID)
	fmt.Printf("ContainerExecInspect >> pid[%d]running[%t]\n", execInspect.Pid, execInspect.Running)
	time.Sleep(10 * time.Second)
	for execInspect.Running {
		time.Sleep(1000 * time.Millisecond)
		execInspect, err = cli.ContainerExecInspect(ctx, response.ID)
		fmt.Printf("ContainerExecInspect >> pid[%d]running[%t]\n", execInspect.Pid, execInspect.Running)
	}
	return nil
}
func uploadFile(w http.ResponseWriter, r *http.Request) {
        // Maximum upload of 16 MB files
        r.ParseMultipartForm(16 << 20)

        var fileName string
        for k, v := range r.MultipartForm.File{
                fileName = k
                fmt.Println(k)
                fmt.Println(v)
                break
        }

        // Get handler for filename, size and headers
        file, handler, err := r.FormFile(fileName)
        if err != nil {
                fmt.Println("Error Retrieving the File")
                fmt.Println(err)
                return
        }

        defer file.Close()
        // fmt.Printf("Uploaded File: %+v\n", handler.Filename)
        // fmt.Printf("File Size: %+v\n", handler.Size)
        // fmt.Printf("MIME Header: %+v\n", handler.Header)

        // Create file
        dst, err := os.Create("/files/upload/" + handler.Filename)
        defer dst.Close()
        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }

        // Copy the uploaded file to the created file on the filesystem
        if _, err := io.Copy(dst, file); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }

        fmt.Printf("Successfully Uploaded File [%s] size[%d]\n", handler.Filename, handler.Size)
        fmt.Fprintf(w, "Successfully Uploaded File [%s] size[%d]\n", handler.Filename, handler.Size)
	err = dockerExecSendFax(handler.Filename)
        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }
}


func uploadHandler(w http.ResponseWriter, r *http.Request) {
        ua := r.Header.Get("User-Agent")
        m := "upload"
        fmt.Printf("[%s] %s...\n", ua, m)

        switch r.Method {
        case "GET":
                display_ui(w, "upload", nil)
        case "POST":
                uploadFile(w, r)
        }
}

func cmdExec(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	s := r.FormValue("cmd")
	uuid, err := cmdCreate(s, &cmdQ, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError )
		return
	}
	w.WriteHeader(200)
	w.Write([]byte("<html><a href=\"http://"+os.Getenv("LOCAL_IP")+":8080/res?id="+uuid+"\">check report for "+uuid+"</a></html>"))
	fmt.Printf("adding command to the queue uuid:%s \n", uuid)
	return
}

func checkFileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !errors.Is(error, os.ErrNotExist)
}

func cmdHandler(w http.ResponseWriter, r *http.Request) {
	ua := r.Header.Get("User-Agent")
	m := "cmd"
	fmt.Printf("[%s] %s...\n", ua, m)
	switch r.Method {
	case "GET":
		display(w, "cmd")
	case "POST":
		cmdExec(w, r)
	}
}

func statsInit(s *Stat, latency int32) {
	s.Stdev = float32(0.0);
	s.m2 = float64(0.0);
	s.Max = latency;
	s.Min = latency;
	s.Average = float32(latency);
	s.Count = 1;
}

func statsUpdate(s *Stat, latency int32) {
	if s.Count == 0 {
		statsInit(s, latency)
		return
	}
	s.Count++
	if s.Min > latency {
		s.Min = latency;
	}
	if s.Max < latency {
		s.Max = latency;
	}

	delta := latency - int32(s.Average);
	var c int32
	if s.Count > 0 {
		c = s.Count
	} else {
		c = 1
	}
	s.Average += float32(delta / c)
	delta2 := latency - int32(s.Average);
	s.m2 += float64(delta*delta2);
	if s.Count-1 > 0 {
		c = s.Count-1
	} else {
		c = 1
	}
	s.Stdev = float32(math.Round((math.Sqrt(s.m2 / float64(c)))*100)/100)
}

func resProcessResultFile(fn string, report *Report) (error) {
	file, err := os.Open("/output/"+fn)
	if err != nil {
		fmt.Printf("error opening result file [%s]\n", err)
		return err;
	}
	defer file.Close()

	fi, err := os.Stat("/output/"+fn)
	if err != nil {
		return err
	}
	fmt.Printf("opening result file [%s] size[%d]\n", fn, fi.Size())
	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		fmt.Println(scanner.Text())
		var testReport TestReport
		b := []byte(scanner.Text())
		err := json.Unmarshal(b, &testReport)
		if err != nil {
			fmt.Printf("invalid test report[%s][%s]\n", scanner.Text(), err)
			return err
		}
		if testReport.Action == "call" {

			if testReport.SipLatency.Invite100Ms > 0 {
				statsUpdate(&report.Sip.Invite100, testReport.SipLatency.Invite100Ms)
			}
			if testReport.SipLatency.Invite18xMs > 0 {
				statsUpdate(&report.Sip.Invite18x, testReport.SipLatency.Invite18xMs)
			}
			if testReport.SipLatency.Invite200Ms > 0 {
				statsUpdate(&report.Sip.Invite200, testReport.SipLatency.Invite200Ms)
			}

			if len(testReport.RtpStats) > 0 {
				report.Rtp.Tx.Pkt += testReport.RtpStats[0].Tx.Pkt
				report.Rtp.Tx.Kbytes += testReport.RtpStats[0].Tx.Kbytes
				report.Rtp.Tx.Lost += testReport.RtpStats[0].Tx.Loss
				if report.Rtp.Tx.JitterMax < testReport.RtpStats[0].Tx.JitterMax {
					report.Rtp.Tx.JitterMax = testReport.RtpStats[0].Tx.JitterMax
				}
				report.Rtp.Rx.Pkt += testReport.RtpStats[0].Rx.Pkt
				report.Rtp.Rx.Kbytes += testReport.RtpStats[0].Rx.Kbytes
				report.Rtp.Rx.Lost += testReport.RtpStats[0].Rx.Loss
				if report.Rtp.Rx.JitterMax < testReport.RtpStats[0].Rx.JitterMax {
					report.Rtp.Rx.JitterMax = testReport.RtpStats[0].Rx.JitterMax
				}
			}
			report.Calls += 1
			if testReport.ExpectedCauseCode == N2T_CODE {
				if testReport.ToneDetected == 0 && testReport.CauseCode != 200 {
					testReport.Result = "UNREACHABLE"
					report.Failed += 1
				} else {
					testReport.Result = "REACHABLE"
					report.Reachable += 1
				}
			} else if testReport.CauseCode >= 300 {
				report.Failed += 1
			} else if testReport.CauseCode >= 200 && testReport.CauseCode < 300 {
				report.Connected += 1
				report.Duration += testReport.Duration
				report.AvgDuration = float32(report.Duration/report.Calls)
			}

			reportJson, _ := json.Marshal(testReport)
			rmqPublish(string(reportJson), os.Getenv("RMQ_PUB_KEY_DETAILS"))
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("error opening reading result file [%s]\n", err)
		return err
	}
	return nil
}

func cleanUp(uuid string) (error) {
	entries, err := os.ReadDir("/output")
	if err != nil {
		fmt.Printf("error opening result directory [%s]\n", err)
		return err
	}
	for _, e := range entries {
		s := e.Name()
		if len(s) < 20 {
			continue
		}
		if  s[len(s)-5:] == ".json" && strings.Contains(s, uuid) {
			fmt.Printf("cleanUp: %s\n",s)
			e := os.Remove("/output/"+s)
			if e != nil {
				fmt.Printf("file cleanup error [%s][%s]\n", s, e)
				return e
			}
		}
	}
	return nil
}

func resGetReport(uuid string) (string, error) {
	var report Report
	report.Uuid = uuid
	entries, err := os.ReadDir("/output")
	if err != nil {
		fmt.Printf("error opening result directory [%s]\n", err)
		return "", err
	}
	for _, e := range entries {
		s := e.Name()
		if len(s) < 20 {
			continue
		}
		if  s[len(s)-5:] == ".json" && strings.Contains(s, uuid) {
			fmt.Printf("resGetReport: %s\n",s)
			err := resProcessResultFile(s, &report)
			if err != nil {
				return "", err
			}
		}
	}
	reportJson, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	fmt.Println(string(reportJson))
	return string(reportJson), nil
}

func resHandler(w http.ResponseWriter, r *http.Request) {
	ua := r.Header.Get("User-Agent")
	m := "res"
	fmt.Printf("[%s] %s...\n", ua, m)

	uuid := r.URL.Query().Get("id")

	if uuid == "" {
		fmt.Printf("missing id parameter\n")
		http.Error(w, "missing id parameter", http.StatusInternalServerError)
		return
	}
	fmt.Println("id =>", uuid)

	report, err := resGetReport(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, report)
}

func cmdExecCall(CallsParams []CallParams) (error) {
	xmlActions := ""
	uuid := CallsParams[0].Uuid
	portSip := CallsParams[0].PortSip
	portRtp := CallsParams[0].PortRtp
	idx := CallsParams[0].Idx
	fmt.Printf(">>>> idx:%d\n", idx)
	ipAddr := CallsParams[0].IpAddr
	boundAddr := CallsParams[0].BoundAddr
	callCount := 0
	waitDuration := 0
	for _, p := range CallsParams {
		earlyRecord := "false";
		maxRingingDuration := 0
		if (p.EarlyRecord == 1) {
			earlyRecord = "true"
			maxRingingDuration = 8
			p.Duration=1
		}
		xml := fmt.Sprintf(`<action type="call" label="%s"
	    transport="udp"
	    expected_cause_code="%d"
	    caller="%s@noreply.com"
	    callee="%s"
	    to_uri="%s"
	    repeat="%d"
	    username="%s"
	    password="%s"
	    max_duration="%d" hangup="%d"
	    username="VP_ENV_USERNAME"
	    password="VP_ENV_PASSWORD"
	    rtp_stats="true"
	    record_early="%s"
	    max_ringing_duration="%d"
	    play="/git/voip_patrol/voice_ref_files/reference_8000.wav"
	>
	<x-header name="X-Foo" value="Bar"/>
	</action>`, p.Uuid, p.ExpectedCauseCode, p.From, p.Ruri, p.Ruri, p.Repeat, p.Username, p.Password, p.Duration+2, p.Duration, earlyRecord, maxRingingDuration)
		xmlActions = xmlActions + xml
		callCount = callCount + p.Repeat + 1
		if waitDuration < p.Duration {
			waitDuration = p.Duration
		}
	}
	xml := fmt.Sprintf(`
<config>
  <actions>
    <action type="codec" disable="all"/>
    <action type="codec" enable="pcmu" priority="248"/>
    <action type="codec" enable="pcma" priority="247"/>
    %s
    <action type="wait" complete="true" ms="%d"/>
 </actions>
</config>
	`, xmlActions, (waitDuration + 30)*1000)
	fmt.Printf("%s\n", xml)

	err := createXmlFile(uuid, idx, xml)
	if err != nil {
		return err
	}
	err = cmdDockerExec(uuid, idx, callCount, portSip, portRtp, ipAddr, boundAddr)
	if err != nil {
		return err
	}
	return nil
}

func rmqTest() {
	if tested {
		return
	}
	tested = true
	body := fmt.Sprintf(`
{
    "call": {
       "destination": "x@%s:%s",
       "username": "default",
       "password": "default",
       "count": 2,
       "duration": 10
    }
}`, os.Getenv("VP_SERVER_IP"), os.Getenv("VP_SERVER_PORT"))
	go rmqPublish(body, os.Getenv("RMQ_SUB_KEY_COMMAND"));
}

func cmdRunner() {
	fmt.Printf("runner reading command queue\n")
	for {
		if len(cmdQ) > 0 {
			cmd := cmdQ[len(cmdQ)-1]
			if count + totalActiveCalls > maxCalls {
				fmt.Printf("too many active calls, skipping queue %d > %d (max calls) \n", count, maxCalls)
			}
			cmdQ = cmdQ[:len(cmdQ)-1]
			fmt.Printf("cmd: %v\n", cmd)
			cmdMakeCalls(cmd)
			fmt.Printf("getting command from the queue uuid:%s \n", cmd.Uuid)
			// for cmdIsCallsLeft(cmd.Uuid) {
			if runnersActive() {
				time.Sleep(500 * time.Millisecond)
			}
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func main() {
	version := "0.0.0"
	cmdQ = make([]Cmd, 0)
	cmdCallLeftCount = make(map[string]int)
	go rmqSubscribe(&cmdQ, os.Getenv("RMQ_SUB_Q_CUSTOMER"));
	go rmqSubscribe(&cmdQ, os.Getenv("RMQ_SUB_Q_PROVIDER"));

	maxCalls = 20
	if len(os.Args) < 2 {
		fmt.Printf("Missing argument %d\n", len(os.Args))
		return
	}
	port, e := strconv.Atoi(os.Args[1])
	if e != nil {
		fmt.Printf("Invalid argument port %s\n", os.Args[1])
		return
	}

	// Upload route
	http.HandleFunc("/cmd", cmdHandler)
	http.HandleFunc("/res", resHandler)
        http.HandleFunc("/upload", uploadHandler)

	// http.HandleFunc("/download", downloadHandler)

	go cmdRunner()

	fmt.Printf("version[%s] Listen on port %d\n", version, port)
	e = http.ListenAndServe(":"+os.Args[1], nil)
	if e != nil {
		fmt.Printf("ListenAndServe: %s\n", e)
	}
}
