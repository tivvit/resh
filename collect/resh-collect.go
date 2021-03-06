package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/BurntSushi/toml"
	common "github.com/curusarn/resh/common"
	"io/ioutil"
	"log"
	"net/http"
	//	"os"
	//  "os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	usr, _ := user.Current()
	dir := usr.HomeDir
	configPath := filepath.Join(dir, "/.config/resh.toml")
	reshUuidPath := filepath.Join(dir, "/.resh/resh-uuid")

	machineIdPath := "/etc/machine-id"

	var config common.Config
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		log.Fatal("Error reading config:", err)
	}

	cmdLine := flag.String("cmdLine", "", "command line")
	exitCode := flag.Int("exitCode", -1, "exit code")
	shell := flag.String("shell", "", "actual shell")

	// posix variables
	cols := flag.Int("cols", -1, "$COLUMNS")
	lines := flag.Int("lines", -1, "$LINES")

	home := flag.String("home", "", "$HOME")
	lang := flag.String("lang", "", "$LANG")
	lcAll := flag.String("lcAll", "", "$LC_ALL")
	login := flag.String("login", "", "$LOGIN")
	path := flag.String("path", "", "$PATH")
	pwd := flag.String("pwd", "", "$PWD - present working directory")
	pwdAfter := flag.String("pwdAfter", "", "$PWD after command")
	shellEnv := flag.String("shellEnv", "", "$SHELL")
	term := flag.String("term", "", "$TERM")

	// non-posix
	pid := flag.Int("pid", -1, "$PID")
	shellPid := flag.Int("shellPid", -1,
		"$$ (pid but subshells don't affect it)")
	windowId := flag.Int("windowId", -1, "$WINDOWID - session id")
	shlvl := flag.Int("shlvl", -1, "$SHLVL")

	host := flag.String("host", "", "$HOSTNAME")
	hosttype := flag.String("hosttype", "", "$HOSTTYPE")
	ostype := flag.String("ostype", "", "$OSTYPE")
	machtype := flag.String("machtype", "", "$MACHTYPE")
	gitCdup := flag.String("gitCdup", "", "git rev-parse --show-cdup")
	gitRemote := flag.String("gitRemote", "", "git remote get-url origin")

	gitCdupExitCode := flag.Int("gitCdupExitCode", -1, "... $?")
	gitRemoteExitCode := flag.Int("gitRemoteExitCode", -1, "... $?")

	// before after
	timezoneBefore := flag.String("timezoneBefore", "", "")
	timezoneAfter := flag.String("timezoneAfter", "", "")

	osReleaseId := flag.String("osReleaseId", "", "/etc/os-release ID")
	osReleaseVersionId := flag.String("osReleaseVersionId", "",
		"/etc/os-release ID")
	osReleaseIdLike := flag.String("osReleaseIdLike", "", "/etc/os-release ID")
	osReleaseName := flag.String("osReleaseName", "", "/etc/os-release ID")
	osReleasePrettyName := flag.String("osReleasePrettyName", "",
		"/etc/os-release ID")

	rtb := flag.String("realtimeBefore", "-1", "before $EPOCHREALTIME")
	rta := flag.String("realtimeAfter", "-1", "after $EPOCHREALTIME")
	rtsess := flag.String("realtimeSession", "-1",
		"on session start $EPOCHREALTIME")
	rtsessboot := flag.String("realtimeSessSinceBoot", "-1",
		"on session start $EPOCHREALTIME")
	flag.Parse()

	realtimeAfter, err := strconv.ParseFloat(*rta, 64)
	if err != nil {
		log.Fatal("Flag Parsing error (rta):", err)
	}
	realtimeBefore, err := strconv.ParseFloat(*rtb, 64)
	if err != nil {
		log.Fatal("Flag Parsing error (rtb):", err)
	}
	realtimeSessionStart, err := strconv.ParseFloat(*rtsess, 64)
	if err != nil {
		log.Fatal("Flag Parsing error (rt sess):", err)
	}
	realtimeSessSinceBoot, err := strconv.ParseFloat(*rtsessboot, 64)
	if err != nil {
		log.Fatal("Flag Parsing error (rt sess boot):", err)
	}
	realtimeDuration := realtimeAfter - realtimeBefore
	realtimeSinceSessionStart := realtimeBefore - realtimeSessionStart
	realtimeSinceBoot := realtimeSessSinceBoot + realtimeSinceSessionStart

	timezoneBeforeOffset := getTimezoneOffsetInSeconds(*timezoneBefore)
	timezoneAfterOffset := getTimezoneOffsetInSeconds(*timezoneAfter)
	realtimeBeforeLocal := realtimeBefore + timezoneBeforeOffset
	realtimeAfterLocal := realtimeAfter + timezoneAfterOffset

	realPwd, err := filepath.EvalSymlinks(*pwd)
	if err != nil {
		log.Println("err while handling pwd realpath:", err)
		realPwd = ""
	}
	realPwdAfter, err := filepath.EvalSymlinks(*pwdAfter)
	if err != nil {
		log.Println("err while handling pwdAfter realpath:", err)
		realPwd = ""
	}

	gitDir, gitRealDir := getGitDirs(*gitCdup, *gitCdupExitCode, *pwd)
	if *gitRemoteExitCode != 0 {
		*gitRemote = ""
	}

	if *osReleaseId == "" {
		*osReleaseId = "linux"
	}
	if *osReleaseName == "" {
		*osReleaseName = "Linux"
	}
	if *osReleasePrettyName == "" {
		*osReleasePrettyName = "Linux"
	}

	rec := common.Record{
		// core
		CmdLine:  *cmdLine,
		ExitCode: *exitCode,
		Shell:    *shell,

		// posix
		Cols:  *cols,
		Lines: *lines,

		Home:     *home,
		Lang:     *lang,
		LcAll:    *lcAll,
		Login:    *login,
		Path:     *path,
		Pwd:      *pwd,
		PwdAfter: *pwdAfter,
		ShellEnv: *shellEnv,
		Term:     *term,

		// non-posix
		RealPwd:      realPwd,
		RealPwdAfter: realPwdAfter,
		Pid:          *pid,
		ShellPid:     *shellPid,
		WindowId:     *windowId,
		Host:         *host,
		Hosttype:     *hosttype,
		Ostype:       *ostype,
		Machtype:     *machtype,
		Shlvl:        *shlvl,

		// before after
		TimezoneBefore: *timezoneBefore,
		TimezoneAfter:  *timezoneAfter,

		RealtimeBefore:      realtimeBefore,
		RealtimeAfter:       realtimeAfter,
		RealtimeBeforeLocal: realtimeBeforeLocal,
		RealtimeAfterLocal:  realtimeAfterLocal,

		RealtimeDuration:          realtimeDuration,
		RealtimeSinceSessionStart: realtimeSinceSessionStart,
		RealtimeSinceBoot:         realtimeSinceBoot,

		GitDir:          gitDir,
		GitRealDir:      gitRealDir,
		GitOriginRemote: *gitRemote,
		MachineId:       readFileContent(machineIdPath),
		ReshUuid:        readFileContent(reshUuidPath),

		OsReleaseId:         *osReleaseId,
		OsReleaseVersionId:  *osReleaseVersionId,
		OsReleaseIdLike:     *osReleaseIdLike,
		OsReleaseName:       *osReleaseName,
		OsReleasePrettyName: *osReleasePrettyName,
	}
	sendRecord(rec, strconv.Itoa(config.Port))
}

func sendRecord(r common.Record, port string) {
	recJson, err := json.Marshal(r)
	if err != nil {
		log.Fatal("send err 1", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:"+port+"/record",
		bytes.NewBuffer(recJson))
	if err != nil {
		log.Fatal("send err 2", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		log.Fatal("resh-daemon is not running :(")
	}
}

func readFileContent(path string) string {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("failed to open " + path)
	}
	return strings.TrimSuffix(string(dat), "\n")
}

func getGitDirs(cdup string, exitCode int, pwd string) (string, string) {
	if exitCode != 0 {
		return "", ""
	}
	abspath := filepath.Clean(filepath.Join(pwd, cdup))
	realpath, err := filepath.EvalSymlinks(abspath)
	if err != nil {
		log.Println("err while handling git dir paths:", err)
		return "", ""
	}
	return abspath, realpath
}

func getTimezoneOffsetInSeconds(zone string) float64 {
	hours_mins := strings.Split(zone, ":")
	hours, err := strconv.Atoi(hours_mins[0])
	if err != nil {
		log.Println("err while parsing hours in timezone offset:", err)
		return -1
	}
	mins, err := strconv.Atoi(hours_mins[1])
	if err != nil {
		log.Println("err while parsing mins in timezone offset:", err)
		return -1
	}
	secs := ((hours * 60) + mins) * 60
	return float64(secs)
}

// func getGitRemote() string {
// 	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
// 	if err != nil {
// 		if exitError, ok := err.(*exec.ExitError); ok {
// 			if exitError.ExitCode() == 128 {
// 				return ""
// 			}
// 			log.Fatal("git remote cmd failed")
// 		} else {
// 			log.Fatal("git remote cmd failed w/o exit code")
// 		}
// 	}
// 	return strings.TrimSuffix(string(out), "\n")
// }
//
// func getGitDir() string {
// 	// assume we are in pwd
// 	gitWorkTree := os.Getenv("GIT_WORK_TREE")
//
// 	if gitWorkTree != "" {
// 		return gitWorkTree
// 	}
// 	// we should look up the git directory ourselves
// 	// OR leave it to resh daemon to not slow down user
// 	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
// 	if err != nil {
// 		if exitError, ok := err.(*exec.ExitError); ok {
// 			if exitError.ExitCode() == 128 {
// 				return ""
// 			}
// 			log.Fatal("git rev-parse cmd failed")
// 		} else {
// 			log.Fatal("git rev-parse cmd failed w/o exit code")
// 		}
// 	}
// 	return strings.TrimSuffix(string(out), "\n")
// }
