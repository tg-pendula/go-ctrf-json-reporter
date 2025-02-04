package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tg-pendula/go-ctrf-json-reporter/ctrf"
)

type TestEvent struct {
	Time    string
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

var coverageRegex *regexp.Regexp = regexp.MustCompile(`coverage: (\d+\.\d+)`)

type suiteCoverage struct {
	Suite    string  `json:"suite"`
	Coverage float32 `json:"coverage"`
}

func getCoverage(event TestEvent) (*suiteCoverage, error) {
	if subMatches := coverageRegex.FindAllStringSubmatch(event.Output, 1); len(subMatches) > 0 && len(subMatches[0]) == 2 {
		// the first subMatch will be: ["coverage: 94.1", "94.1"] (we can always expect 2 items)

		f, err := strconv.ParseFloat(subMatches[0][1], 32)

		if err != nil {
			return nil, fmt.Errorf("error parsing coverage: %w", err)
		}

		return &suiteCoverage{
			Suite:    event.Package,
			Coverage: float32(f),
		}, nil
	}

	return nil, nil
}

func ParseTestResults(r io.Reader, verbose bool, env *ctrf.Environment) (*ctrf.Report, error) {
	var testEvents []TestEvent
	decoder := json.NewDecoder(r)

	for {
		var event TestEvent
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		testEvents = append(testEvents, event)
	}

	report := ctrf.NewReport("gotest", env)
	report.Results.Summary.Start = time.Now().UnixNano() / int64(time.Millisecond)

	// Coverage is reported twice when tests exist for a package, but only once when no tests exist, so use a map to collect unique results
	coverage := map[string]suiteCoverage{}

	for i, event := range testEvents {
		if verbose {
			jsonEvent, err := json.Marshal(event)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			fmt.Println(string(jsonEvent))
		}

		startTime, err := parseTimeString(event.Time)
		duration := secondsToMillis(event.Elapsed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing test event start time '%s' : %v\n", event.Time, err)
		} else {
			if report.Results.Summary.Start > startTime {
				report.Results.Summary.Start = startTime
			}
			endTime := startTime + duration
			if report.Results.Summary.Stop < endTime {
				report.Results.Summary.Stop = endTime
			}
		}

		// Events relating to a specific Test
		if len(event.Test) > 0 {
			if event.Action == "pass" {
				report.Results.Summary.Tests++
				report.Results.Summary.Passed++
				report.Results.Tests = append(report.Results.Tests, &ctrf.TestResult{
					Suite:    event.Package,
					Name:     event.Test,
					Status:   ctrf.TestPassed,
					Duration: duration,
				})
			} else if event.Action == "fail" {
				report.Results.Summary.Tests++
				report.Results.Summary.Failed++
				report.Results.Tests = append(report.Results.Tests, &ctrf.TestResult{
					Suite:    event.Package,
					Name:     event.Test,
					Status:   ctrf.TestFailed,
					Duration: duration,
					Message:  getMessagesForTest(testEvents, i, event.Package, event.Test),
				})
			} else if event.Action == "skip" {
				report.Results.Summary.Tests++
				report.Results.Summary.Skipped++
				report.Results.Tests = append(report.Results.Tests, &ctrf.TestResult{
					Suite:    event.Package,
					Name:     event.Test,
					Status:   ctrf.TestSkipped,
					Duration: duration,
				})
			}
		} else {
			if event.Action == "output" {
				pc, err := getCoverage(event)

				if err != nil {
					fmt.Fprintf(os.Stderr, "error parsing coverage '%s' : %v\n", event.Output, err)
					continue
				}

				if pc != nil {
					coverage[pc.Suite] = *pc
				}
			}
		}
	}

	if len(coverage) > 0 {
		addCoverage(report, coverage)
	}

	return report, nil
}

func addCoverage(report *ctrf.Report, coverage map[string]suiteCoverage) {
	// Convert coverage to a list so its easier to consume
	coverageList := make([]suiteCoverage, 0, len(coverage))

	for _, v := range coverage {
		coverageList = append(coverageList, v)
	}

	sort.SliceStable(coverageList, func(i, j int) bool {
		return strings.Compare(coverageList[i].Suite, coverageList[j].Suite) == -1
	})

	report.Results.Summary.Extra = map[string]any{"coverage": coverageList}
}

func getMessagesForTest(testEvents []TestEvent, index int, packageName, testName string) string {
	var messages []string
	for i := index; i >= 0; i-- {
		if testEvents[i].Package == packageName && testEvents[i].Test == testName {
			if testEvents[i].Action == "output" {
				messages = append(messages, testEvents[i].Output)
			}
		} else {
			break
		}
	}
	reverse(messages)
	return strings.Join(messages, "")
}

func WriteReportToFile(filename string, report *ctrf.Report) error {
	err := report.WriteFile(filename)
	if err != nil {
		return err
	}
	fmt.Println("go-ctrf-json-reporter: successfully written ctrf json to", filename)
	return nil
}

func secondsToMillis(seconds float64) int64 {
	return int64(seconds * 1000)
}

func parseTimeString(timeString string) (int64, error) {
	t, err := time.Parse(time.RFC3339Nano, timeString)
	if err != nil {
		return 0, err
	}
	return t.UnixNano() / int64(time.Millisecond), nil
}

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
