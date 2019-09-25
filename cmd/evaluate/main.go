package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/curusarn/resh/pkg/records"
	"github.com/jpillora/longestcommon"

	"github.com/schollz/progressbar"
)

// Version from git set during build
var Version string

// Revision from git set during build
var Revision string

func main() {
	const maxCandidates = 50

	usr, _ := user.Current()
	dir := usr.HomeDir
	historyPath := filepath.Join(dir, ".resh_history.json")
	historyPathBatchMode := filepath.Join(dir, "resh_history.json")
	sanitizedHistoryPath := filepath.Join(dir, "resh_history_sanitized.json")
	// tmpPath := "/tmp/resh-evaluate-tmp.json"

	showVersion := flag.Bool("version", false, "Show version and exit")
	showRevision := flag.Bool("revision", false, "Show git revision and exit")
	input := flag.String("input", "",
		"Input file (default: "+historyPath+"OR"+sanitizedHistoryPath+
			" depending on --sanitized-input option)")
	// outputDir := flag.String("output", "/tmp/resh-evaluate", "Output directory")
	sanitizedInput := flag.Bool("sanitized-input", false,
		"Handle input as sanitized (also changes default value for input argument)")
	plottingScript := flag.String("plotting-script", "resh-evaluate-plot.py", "Script to use for plotting")
	inputDataRoot := flag.String("input-data-root", "",
		"Input data root, enables batch mode, looks for files matching --input option")
	slow := flag.Bool("slow", false,
		"Enables strategies that takes a long time (e.g. markov chain strategies).")
	skipFailedCmds := flag.Bool("skip-failed-cmds", false,
		"Skips records with non-zero exit status.")
	debugRecords := flag.Float64("debug", 0, "Debug records - percentage of records that should be debugged.")

	flag.Parse()

	// handle show{Version,Revision} options
	if *showVersion == true {
		fmt.Println(Version)
		os.Exit(0)
	}
	if *showRevision == true {
		fmt.Println(Revision)
		os.Exit(0)
	}

	// handle batch mode
	batchMode := false
	if *inputDataRoot != "" {
		batchMode = true
	}
	// set default input
	if *input == "" {
		if *sanitizedInput {
			*input = sanitizedHistoryPath
		} else if batchMode {
			*input = historyPathBatchMode
		} else {
			*input = historyPath
		}
	}

	evaluator := evaluator{sanitizedInput: *sanitizedInput, maxCandidates: maxCandidates,
		BatchMode: batchMode, skipFailedCmds: *skipFailedCmds, debugRecords: *debugRecords}
	if batchMode {
		err := evaluator.initBatchMode(*input, *inputDataRoot)
		if err != nil {
			log.Fatal("Evaluator initBatchMode() error:", err)
		}
	} else {
		err := evaluator.init(*input)
		if err != nil {
			log.Fatal("Evaluator init() error:", err)
		}
	}

	var simpleStrategies []ISimpleStrategy
	var strategies []IStrategy

	// dummy := strategyDummy{}
	// simpleStrategies = append(simpleStrategies, &dummy)

	simpleStrategies = append(simpleStrategies, &strategyRecent{})

	// frequent := strategyFrequent{}
	// frequent.init()
	// simpleStrategies = append(simpleStrategies, &frequent)

	// random := strategyRandom{candidatesSize: maxCandidates}
	// random.init()
	// simpleStrategies = append(simpleStrategies, &random)

	directory := strategyDirectorySensitive{}
	directory.init()
	simpleStrategies = append(simpleStrategies, &directory)

	dynamicDistG := strategyDynamicRecordDistance{
		maxDepth:   3000,
		distParams: records.DistParams{Pwd: 10, RealPwd: 10, SessionID: 1, Time: 1, Git: 10},
		label:      "10*pwd,10*realpwd,session,time,10*git",
	}
	dynamicDistG.init()
	strategies = append(strategies, &dynamicDistG)

	distanceStaticBest := strategyRecordDistance{
		maxDepth:   3000,
		distParams: records.DistParams{Pwd: 10, RealPwd: 10, SessionID: 1, Time: 1},
		label:      "10*pwd,10*realpwd,session,time",
	}
	strategies = append(strategies, &distanceStaticBest)

	recentBash := strategyRecentBash{}
	recentBash.init()
	strategies = append(strategies, &recentBash)

	if *slow {

		markovCmd := strategyMarkovChainCmd{order: 1}
		markovCmd.init()

		markovCmd2 := strategyMarkovChainCmd{order: 2}
		markovCmd2.init()

		markov := strategyMarkovChain{order: 1}
		markov.init()

		markov2 := strategyMarkovChain{order: 2}
		markov2.init()

		simpleStrategies = append(simpleStrategies, &markovCmd2, &markovCmd, &markov2, &markov)
	}

	for _, strat := range simpleStrategies {
		strategies = append(strategies, NewSimpleStrategyWrapper(strat))
	}

	for _, strat := range strategies {
		err := evaluator.evaluate(strat)
		if err != nil {
			log.Println("Evaluator evaluate() error:", err)
		}
	}

	evaluator.calculateStatsAndPlot(*plottingScript)
}

type ISimpleStrategy interface {
	GetTitleAndDescription() (string, string)
	GetCandidates() []string
	AddHistoryRecord(record *records.EnrichedRecord) error
	ResetHistory() error
}

type IStrategy interface {
	GetTitleAndDescription() (string, string)
	GetCandidates(r records.EnrichedRecord) []string
	AddHistoryRecord(record *records.EnrichedRecord) error
	ResetHistory() error
}

type simpleStrategyWrapper struct {
	strategy ISimpleStrategy
}

// NewSimpleStrategyWrapper returns IStrategy created by wrapping given ISimpleStrategy
func NewSimpleStrategyWrapper(strategy ISimpleStrategy) *simpleStrategyWrapper {
	return &simpleStrategyWrapper{strategy: strategy}
}

func (s *simpleStrategyWrapper) GetTitleAndDescription() (string, string) {
	return s.strategy.GetTitleAndDescription()
}

func (s *simpleStrategyWrapper) GetCandidates(r records.EnrichedRecord) []string {
	return s.strategy.GetCandidates()
}

func (s *simpleStrategyWrapper) AddHistoryRecord(r *records.EnrichedRecord) error {
	return s.strategy.AddHistoryRecord(r)
}

func (s *simpleStrategyWrapper) ResetHistory() error {
	return s.strategy.ResetHistory()
}

type matchJSON struct {
	Match         bool
	Distance      int
	CharsRecalled int
}

type multiMatchItemJSON struct {
	Distance      int
	CharsRecalled int
}

type multiMatchJSON struct {
	Match   bool
	Entries []multiMatchItemJSON
}

type strategyJSON struct {
	Title         string
	Description   string
	Matches       []matchJSON
	PrefixMatches []multiMatchJSON
}

type deviceRecords struct {
	Name    string
	Records []records.EnrichedRecord
}

type userRecords struct {
	Name    string
	Devices []deviceRecords
}

type evaluator struct {
	sanitizedInput bool
	BatchMode      bool
	maxCandidates  int
	skipFailedCmds bool
	debugRecords   float64
	UsersRecords   []userRecords
	Strategies     []strategyJSON
}

func (e *evaluator) initBatchMode(input string, inputDataRoot string) error {
	e.UsersRecords = e.loadHistoryRecordsBatchMode(input, inputDataRoot)
	e.preprocessRecords()
	return nil
}

func (e *evaluator) init(inputPath string) error {
	records := e.loadHistoryRecords(inputPath)
	device := deviceRecords{Records: records}
	user := userRecords{}
	user.Devices = append(user.Devices, device)
	e.UsersRecords = append(e.UsersRecords, user)
	e.preprocessRecords()
	return nil
}

func (e *evaluator) calculateStatsAndPlot(scriptName string) {
	evalJSON, err := json.Marshal(e)
	if err != nil {
		log.Fatal("json marshal error", err)
	}
	buffer := bytes.Buffer{}
	buffer.Write(evalJSON)
	// run python script to stat and plot/
	cmd := exec.Command(scriptName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = &buffer
	err = cmd.Run()
	if err != nil {
		log.Printf("Command finished with error: %v", err)
	}
}

func (e *evaluator) preprocessDeviceRecords(device deviceRecords) deviceRecords {
	sessionIDs := map[string]uint64{}
	var nextID uint64
	nextID = 1 // start with 1 because 0 won't get saved to json
	for k, record := range device.Records {
		id, found := sessionIDs[record.SessionID]
		if found == false {
			id = nextID
			sessionIDs[record.SessionID] = id
			nextID++
		}
		device.Records[k].SeqSessionID = id
		// assert
		if record.Sanitized != e.sanitizedInput {
			if e.sanitizedInput {
				log.Fatal("ASSERT failed: '--sanitized-input' is present but data is not sanitized")
			}
			log.Fatal("ASSERT failed: data is sanitized but '--sanitized-input' is not present")
		}
		device.Records[k].SeqSessionID = id
		if e.debugRecords > 0 && rand.Float64() < e.debugRecords {
			device.Records[k].DebugThisRecord = true
		}
	}
	// sort.SliceStable(device.Records, func(x, y int) bool {
	// 	if device.Records[x].SeqSessionID == device.Records[y].SeqSessionID {
	// 		return device.Records[x].RealtimeAfterLocal < device.Records[y].RealtimeAfterLocal
	// 	}
	// 	return device.Records[x].SeqSessionID < device.Records[y].SeqSessionID
	// })

	// iterate from back and mark last record of each session
	sessionIDSet := map[string]bool{}
	for i := len(device.Records) - 1; i >= 0; i-- {
		var record *records.EnrichedRecord
		record = &device.Records[i]
		if sessionIDSet[record.SessionID] {
			continue
		}
		sessionIDSet[record.SessionID] = true
		record.LastRecordOfSession = true
	}
	return device
}

// enrich records and add sequential session ID
func (e *evaluator) preprocessRecords() {
	for i := range e.UsersRecords {
		for j := range e.UsersRecords[i].Devices {
			e.UsersRecords[i].Devices[j] = e.preprocessDeviceRecords(e.UsersRecords[i].Devices[j])
		}
	}
}

func (e *evaluator) evaluate(strategy IStrategy) error {
	title, description := strategy.GetTitleAndDescription()
	log.Println("Evaluating strategy:", title, "-", description)
	strategyData := strategyJSON{Title: title, Description: description}
	for i := range e.UsersRecords {
		for j := range e.UsersRecords[i].Devices {
			bar := progressbar.New(len(e.UsersRecords[i].Devices[j].Records))
			var prevRecord records.EnrichedRecord
			for _, record := range e.UsersRecords[i].Devices[j].Records {
				if e.skipFailedCmds && record.ExitCode != 0 {
					continue
				}
				candidates := strategy.GetCandidates(records.Stripped(record))
				if record.DebugThisRecord {
					log.Println()
					log.Println("===================================================")
					log.Println("STRATEGY:", title, "-", description)
					log.Println("===================================================")
					log.Println("Previous record:")
					if prevRecord.RealtimeBefore == 0 {
						log.Println("== NIL")
					} else {
						rec, _ := prevRecord.ToString()
						log.Println(rec)
					}
					log.Println("---------------------------------------------------")
					log.Println("Recommendations for:")
					rec, _ := record.ToString()
					log.Println(rec)
					log.Println("---------------------------------------------------")
					for i, candidate := range candidates {
						if i > 10 {
							break
						}
						log.Println(string(candidate))
					}
					log.Println("===================================================")
				}

				matchFound := false
				longestPrefixMatchLength := 0
				multiMatch := multiMatchJSON{}
				for i, candidate := range candidates {
					// make an option (--calculate-total) to turn this on/off ?
					// if i >= e.maxCandidates {
					// 	break
					// }
					commonPrefixLength := len(longestcommon.Prefix([]string{candidate, record.CmdLine}))
					if commonPrefixLength > longestPrefixMatchLength {
						longestPrefixMatchLength = commonPrefixLength
						prefixMatch := multiMatchItemJSON{Distance: i + 1, CharsRecalled: commonPrefixLength}
						multiMatch.Match = true
						multiMatch.Entries = append(multiMatch.Entries, prefixMatch)
					}
					if candidate == record.CmdLine {
						match := matchJSON{Match: true, Distance: i + 1, CharsRecalled: record.CmdLength}
						matchFound = true
						strategyData.Matches = append(strategyData.Matches, match)
						strategyData.PrefixMatches = append(strategyData.PrefixMatches, multiMatch)
						break
					}
				}
				if matchFound == false {
					strategyData.Matches = append(strategyData.Matches, matchJSON{})
					strategyData.PrefixMatches = append(strategyData.PrefixMatches, multiMatch)
				}
				err := strategy.AddHistoryRecord(&record)
				if err != nil {
					log.Println("Error while evauating", err)
					return err
				}
				bar.Add(1)
				prevRecord = record
			}
			strategy.ResetHistory()
			fmt.Println()
		}
	}
	e.Strategies = append(e.Strategies, strategyData)
	return nil
}

func (e *evaluator) loadHistoryRecordsBatchMode(fname string, dataRootPath string) []userRecords {
	var records []userRecords
	info, err := os.Stat(dataRootPath)
	if err != nil {
		log.Fatal("Error: Directory", dataRootPath, "does not exist - exiting! (", err, ")")
	}
	if info.IsDir() == false {
		log.Fatal("Error:", dataRootPath, "is not a directory - exiting!")
	}
	users, err := ioutil.ReadDir(dataRootPath)
	if err != nil {
		log.Fatal("Could not read directory:", dataRootPath)
	}
	fmt.Println("Listing users in <", dataRootPath, ">...")
	for _, user := range users {
		userRecords := userRecords{Name: user.Name()}
		userFullPath := filepath.Join(dataRootPath, user.Name())
		if user.IsDir() == false {
			log.Println("Warn: Unexpected file (not a directory) <", userFullPath, "> - skipping.")
			continue
		}
		fmt.Println()
		fmt.Printf("*- %s\n", user.Name())
		devices, err := ioutil.ReadDir(userFullPath)
		if err != nil {
			log.Fatal("Could not read directory:", userFullPath)
		}
		for _, device := range devices {
			deviceRecords := deviceRecords{Name: device.Name()}
			deviceFullPath := filepath.Join(userFullPath, device.Name())
			if device.IsDir() == false {
				log.Println("Warn: Unexpected file (not a directory) <", deviceFullPath, "> - skipping.")
				continue
			}
			fmt.Printf("   \\- %s\n", device.Name())
			files, err := ioutil.ReadDir(deviceFullPath)
			if err != nil {
				log.Fatal("Could not read directory:", deviceFullPath)
			}
			for _, file := range files {
				fileFullPath := filepath.Join(deviceFullPath, file.Name())
				if file.Name() == fname {
					fmt.Printf("      \\- %s - loading ...", file.Name())
					// load the data
					deviceRecords.Records = e.loadHistoryRecords(fileFullPath)
					fmt.Println(" OK ✓")
				} else {
					fmt.Printf("      \\- %s - skipped\n", file.Name())
				}
			}
			userRecords.Devices = append(userRecords.Devices, deviceRecords)
		}
		records = append(records, userRecords)
	}
	return records
}

func (e *evaluator) loadHistoryRecords(fname string) []records.EnrichedRecord {
	file, err := os.Open(fname)
	if err != nil {
		log.Fatal("Open() resh history file error:", err)
	}
	defer file.Close()

	var recs []records.EnrichedRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		record := records.Record{}
		fallbackRecord := records.FallbackRecord{}
		line := scanner.Text()
		err = json.Unmarshal([]byte(line), &record)
		if err != nil {
			err = json.Unmarshal([]byte(line), &fallbackRecord)
			if err != nil {
				log.Println("Line:", line)
				log.Fatal("Decoding error:", err)
			}
			record = records.ConvertRecord(&fallbackRecord)
		}
		if e.sanitizedInput == false {
			if record.CmdLength != 0 {
				log.Fatal("Assert failed - 'cmdLength' is set in raw data. Maybe you want to use '--sanitized-input' option?")
			}
			record.CmdLength = len(record.CmdLine)
		}
		if record.CmdLength == 0 {
			log.Fatal("Assert failed - 'cmdLength' is unset in the data. This should not happen.")
		}
		recs = append(recs, record.Enrich())
	}
	return recs
}
