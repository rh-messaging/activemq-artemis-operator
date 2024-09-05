package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/google/shlex"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"k8s.io/utils/strings/slices"
)

type RunningCommand struct {
	env           []string
	cmd           *exec.Cmd
	outb          bytes.Buffer
	errb          bytes.Buffer
	cmdPrettyName string
	cancelFunc    context.CancelFunc
	stdout        string
	stderr        string
	returnCode    int
	isBash        bool
}

type ExecutableChunk struct {
	Stage         string `json:"stage"`
	Id            string `json:"id,omitempty"`
	Requires      string `json:"requires,omitempty"`
	RootDir       string `json:"rootdir,omitempty"`
	Runtime       string `json:"runtime,omitempty"`
	IsParallel    bool   `json:"parallel,omitempty"`
	Label         string `json:"label,omitempty"`
	HasBreakpoint bool   `json:"breakpoint,omitempty"`
	Destination   string `json:"destination,omitempty"`
	content       []string
	commands      []*RunningCommand
	backQuotes    int
}

const CHUNK_REGEX = "^```+[a-zA-Z0-9_\\-. ]*\\{.*stage.*\\}.*$"
const OUTPUT_CHUNK_REGEX = "^```+shell tutorial_tester$"

var dryRun bool = false
var interactive bool = false
var verbose bool = false
var minutesToTimeout int = 10
var startFrom string = ""
var operator_root = "./"
var ingoreBreakpoints bool = false
var updateTutorials bool = false
var justList bool = false
var noStyling bool = false
var quiet bool = false
var tutorials_root string = ""
var env []string

var schema string = `
{
    "type":"object",
    "properties":{
        "stage":{"type":"string", "pattern":"^[a-zA-Z0-9_-]*$"},
        "id":{"type":"string", "pattern":"^[a-zA-Z0-9_-]*$"},
        "requires":{"type":"string", "pattern":"^[a-zA-Z0-9_-]*/[a-zA-Z0-9_-]*$"},
        "rootdir":{"type":"string", "pattern":"^(\\$operator|\\$tmpdir\\.?\\w*)?[\\w\\/\\-\\.]*$"},
        "runtime":{"enum": ["bash", "writer"]},
        "parallel":{"type":"boolean"},
        "breakpoint":{"type":"boolean"},
        "Destination":{"type":"string", "pattern":"^[\\w\\/\\-\\.]*$"}
    },
    "required":["stage"]
}
`

func initChunk(params string) (*ExecutableChunk, error) {
	var chunk ExecutableChunk
	err := json.Unmarshal([]byte(params), &chunk)
	chunk.content = []string{}
	if chunk.HasBreakpoint {
		pterm.Warning.Println("breakpoint in the document")
	}
	if chunk.Runtime == "writer" {
		if chunk.Destination == "" {
			return nil, errors.New("a writer runtime requires a destination property")
		}
	}
	return &chunk, err
}

func Fail(message string, callerSkip ...int) {
	panic(message)
}

// Parse the tutorial file name and extract all its chunks.
// The `tutorial` has to exist within the `tutorials_root` folder.
func extractStages(tutorial string) [][]*ExecutableChunk {
	var stages [][]*ExecutableChunk
	var err error
	var file *os.File
	filepath := path.Join(tutorials_root, tutorial)
	file, err = os.Open(filepath)
	Expect(err).To(BeNil())
	defer file.Close()
	scanner := bufio.NewScanner(file)
	// regex for the chunk opening fence
	chunkMatcher, err := regexp.Compile(CHUNK_REGEX)
	pterm.Fatal.PrintOnError(err)
	var isInChunk bool = false
	var chunkStopFend *regexp.Regexp // a regex to match when the chunk is ending
	var chunkBackQuotesCount int = 0 // with its number of back quotes

	// regex for output opening fence
	outputChunkMatcer, err := regexp.Compile(OUTPUT_CHUNK_REGEX)
	pterm.Fatal.PrintOnError(err)
	var isInFormerOutputChunk bool = false // when set to true all lines from input are ignored
	var outputChunkStopFend *regexp.Regexp // a regex to match when the chunk is ending
	var outputChunkBackQuotesCount int = 0 // with its number of back quotes

	var currentStage string = ""
	var currentChunk *ExecutableChunk

	var lineCounter = 0

	sch, err := jsonschema.CompileString("schema.json", schema)
	pterm.Fatal.PrintOnError(err)
	for scanner.Scan() {
		lineCounter += 1
		// when we encounter the previous output chunk, we ignore everything until the corresponding fence closing
		if !isInChunk && !isInFormerOutputChunk && outputChunkMatcer.Match(scanner.Bytes()) {
			isInFormerOutputChunk = true
			outputChunkBackQuotesCount = countOpeningBackQuotes(scanner.Text())
			outputChunkStopFend, err = regexp.Compile(fmt.Sprintf("^`{%d}$", outputChunkBackQuotesCount))
			pterm.Fatal.PrintOnError(err)
			continue
		}
		if !isInChunk && isInFormerOutputChunk && outputChunkStopFend.Match(scanner.Bytes()) {
			isInFormerOutputChunk = false
			continue
		}
		// When we detect a chunk we compute how many backticks are needed to find its end
		if !isInChunk && !isInFormerOutputChunk && chunkMatcher.Match(scanner.Bytes()) {
			chunkBackQuotesCount = countOpeningBackQuotes(scanner.Text())
			chunkStopFend, err = regexp.Compile(fmt.Sprintf("^`{%d}$", chunkBackQuotesCount))
			pterm.Fatal.PrintOnError(err)
			isInChunk = true
			raw := scanner.Text()
			params := raw[strings.Index(raw, "{"):]
			var v interface{}
			if err := json.Unmarshal([]byte(params), &v); err != nil {
				pterm.Fatal.Printf(fmt.Sprintf("%s@%d %s in %s\n", filepath, lineCounter, err, params))
			}
			if err = sch.Validate(v); err != nil {
				pterm.Fatal.Printf(fmt.Sprintf("%s@%d %s in %s\n", filepath, lineCounter, err, params))
			}
			currentChunk, err = initChunk(params)
			if err != nil {
				pterm.Fatal.Printf(fmt.Sprintf("%s@%d %s in %s\n", filepath, lineCounter, err, params))
			}
			if currentStage != currentChunk.Stage {
				stages = append(stages, []*ExecutableChunk{})
				currentStage = currentChunk.Stage
			}
			stages[len(stages)-1] = append(stages[len(stages)-1], currentChunk)
			continue
		}
		// when the end is detected, it's time to write the new output
		if isInChunk && !isInFormerOutputChunk {
			if chunkStopFend.Match(scanner.Bytes()) {
				isInChunk = false
			} else {
				currentChunk.content = append(currentChunk.content, scanner.Text())
			}
		}
	}
	return stages
}

// Create/update the output chunks for every executed chunk inside the `tutorial` file.
// The `tutorial` has to exist within the `tutorials_root` folder.
func updateChunkOutput(tutorial string, stages [][]*ExecutableChunk) error {
	// get the path for the input tutorial
	inFPath := path.Join(tutorials_root, tutorial)
	inFile, err := os.Open(inFPath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	// write to a temporary file
	outFPath := path.Join(tutorials_root, tutorial+".out")
	outFile, err := os.Create(outFPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	scanner := bufio.NewScanner(inFile)

	// regex for the chunk opening fence
	chunkMatcher, err := regexp.Compile(CHUNK_REGEX)
	if err != nil {
		return err
	}
	var isInChunk bool = false
	var chunkStopFend *regexp.Regexp // a regex to match when the chunk is ending
	var chunkBackQuotesCount int = 0 // with its number of back quotes

	// regex for output opening fence
	outputChunkMatcer, err := regexp.Compile(OUTPUT_CHUNK_REGEX)
	if err != nil {
		return err
	}
	var isInFormerOutputChunk bool = false // when set to true all lines from input are ignored
	var outputChunkStopFend *regexp.Regexp // a regex to match when the chunk is ending
	var outputChunkBackQuotesCount int = 0 // with its number of back quotes

	var writeNewOutput bool = false // when set to true, the stdout is written in a new output chunk

	var currentStage int = 0
	var currentChunk int = 0

	for scanner.Scan() {
		// when we encounter the previous output chunk, we ignore everything until the corresponding fence closing
		if !isInChunk && !isInFormerOutputChunk && outputChunkMatcer.Match(scanner.Bytes()) {
			isInFormerOutputChunk = true
			outputChunkBackQuotesCount = countOpeningBackQuotes(scanner.Text())
			outputChunkStopFend, err = regexp.Compile(fmt.Sprintf("^`{%d}$", outputChunkBackQuotesCount))
			if err != nil {
				return err
			}
			continue
		}
		if !isInFormerOutputChunk {
			_, err = writer.WriteString(scanner.Text() + "\n")
			if err != nil {
				return err
			}
		}
		if !isInChunk && isInFormerOutputChunk && outputChunkStopFend.Match(scanner.Bytes()) {
			isInFormerOutputChunk = false
		}
		// When we detect a chunk we compute how many backticks are needed to find its end
		if !isInChunk && !isInFormerOutputChunk && chunkMatcher.Match(scanner.Bytes()) {
			chunkBackQuotesCount = countOpeningBackQuotes(scanner.Text())
			chunkStopFend, err = regexp.Compile(fmt.Sprintf("^`{%d}$", chunkBackQuotesCount))
			if err != nil {
				return err
			}
			isInChunk = true
			continue
		}
		// when the end is detected, it's time to write the new output
		if isInChunk && chunkStopFend.Match(scanner.Bytes()) {
			isInChunk = false
			writeNewOutput = true
		}
		if writeNewOutput {
			writeNewOutput = false
			// get the current chunk to write
			stage := stages[currentStage]
			chunk := stage[currentChunk]
			if chunk.hasOutput() {
				chunk.writeOutputTo(chunkBackQuotesCount, writer)
			}
			// Compute the next chunk and stage
			currentChunk += 1
			if currentChunk == len(stages[currentStage]) {
				currentChunk = 0
				currentStage += 1
			}
		}
	}
	return writer.Flush()
}

func (chunk *ExecutableChunk) getOrCreateRuntimeDirectory(tmpDirs map[string]string) (string, error) {
	var terminatingError error
	// if nothing was passed in, create a unique temporary directory anyway to keep the system clean
	if chunk.RootDir == "" {
		uuid := uuid.New()
		chunk.RootDir = "$tmpdir." + uuid.String()
	}
	// Initialize where the command is getting executed
	if chunk.RootDir == "$operator" {
		return operator_root, nil
	}
	// tmpdirs are reusable between commands.
	if strings.HasPrefix(chunk.RootDir, "$tmpdir") {
		dirselector := strings.Split(chunk.RootDir, string(os.PathSeparator))[0]
		// so they are stored in a map.
		tmpdir, exists := tmpDirs[dirselector]
		if !exists {
			tmpdir, terminatingError = os.MkdirTemp("/tmp", "*")
			if terminatingError != nil {
				pterm.Error.PrintOnError(terminatingError)
				return "", terminatingError
			}
			tmpDirs[dirselector] = tmpdir
		}
		return strings.Replace(chunk.RootDir, dirselector, tmpdir, 1), nil
	}
	// if the directory doesn't start with a $ sign the user wanted its own custom value, let's use it directly
	if !strings.HasPrefix(chunk.RootDir, "$") {
		return chunk.RootDir, nil
	}
	return "", errors.New("Impossible to figure out the directory to run in: " + chunk.RootDir)
}

// Finds a chunk by its ID and stage name within all the stages passed as parameters.
func findChunkById(stages [][]*ExecutableChunk, stageName string, chunkId string) *ExecutableChunk {
	for _, stage := range stages {
		if stage[0].Stage != stageName {
			continue
		}
		for _, chunk := range stage {
			if chunk.Id == chunkId {
				return chunk
			}
		}
	}
	return nil
}

func (chunk *ExecutableChunk) writeFile(basedir string) error {
	scriptPath := path.Join(basedir, chunk.Destination)
	f, err := os.Create(scriptPath)
	defer f.Close()
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(f)
	// the actual content the user wants in the file
	for _, line := range chunk.content {
		_, err = writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}
	err = writer.Flush()
	if err != nil {
		return err
	}
	return nil
}

// write the content as a bash script inside the at `basedir/script_name`
// prefix and suffix the script with extra commands
func (chunk *ExecutableChunk) writeBashScript(basedir string, script_name string) error {
	scriptPath := path.Join(basedir, script_name)
	f, err := os.Create(scriptPath)
	defer f.Close()
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(f)
	_, err = writer.WriteString("#!/bin/bash\n")
	if err != nil {
		return err
	}

	// fail fast
	_, err = writer.WriteString("set -euo pipefail\n")
	if err != nil {
		return err
	}

	// the actual script the user wants to execute
	for _, line := range chunk.content {
		_, err = writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	// bubble up the env after the script execution
	_, err = writer.WriteString("echo \"### ENV ###\"\n")
	_, err = writer.WriteString("printenv\n")
	if err != nil {
		return err
	}
	err = writer.Flush()
	if err != nil {
		return err
	}
	return os.Chmod(scriptPath, 0770)
}

// Returns true if the every commands in the chunk has a return code of 0
func (chunk *ExecutableChunk) hasExecutedCorrectly() bool {
	if dryRun {
		return true
	}
	var allOk bool = true
	for _, command := range chunk.commands {
		allOk = allOk && command.cmd.ProcessState.ExitCode() == 0
	}
	return allOk
}

// Returns true if at least one command of the chunk had some output on the stdout pipe
func (chunk *ExecutableChunk) hasOutput() bool {
	if dryRun {
		return true
	}
	var hasOutput bool = len(chunk.commands) > 0
	for _, command := range chunk.commands {
		hasOutput = hasOutput && command.stdout != ""
	}
	return hasOutput
}

// Count how many back quotes are prefixing the string `str`
func countOpeningBackQuotes(str string) int {
	total := 0
	// count the total amount of backquotes to be able to find the closing fence
	after, found := strings.CutPrefix(str, "`")
	for found {
		total += 1
		after, found = strings.CutPrefix(after, "`")
	}
	return total
}

// Write an output chunk on the bufio.Writer from the commands result of the chunk
func (chunk *ExecutableChunk) writeOutputTo(bqNumber int, writer *bufio.Writer) error {
	var err error
	// print the start of the chunk
	for i := 0; i < bqNumber; i++ {
		_, err = writer.WriteString("`")
		if err != nil {
			return err
		}
	}
	_, err = writer.WriteString("shell tutorial_tester\n")
	if err != nil {
		return err
	}
	for _, command := range chunk.commands {
		if command.stdout != "" {
			_, err = writer.WriteString(command.stdout)
			if err != nil {
				return err
			}
			// make sure to only have one carriage return at the end
			if command.stdout[len(command.stdout)-1] != '\n' {
				_, err = writer.WriteString("\n")
				if err != nil {
					return err
				}
			}

		}
	}
	// print the end of the chunk
	for i := 0; i < bqNumber; i++ {
		_, err = writer.WriteString("`")
		if err != nil {
			return err
		}
	}
	// add a final carriage return
	_, err = writer.WriteString("\n")
	if err != nil {
		return err
	}
	return nil
}

// Parse the received command string and turn it into a exec.Cmd instance that can be start later on
// If the chunk is a bash script, the content of the chunk is written to a file on disk
// The exec.Cmd instance is Initialized with the environment of it's predecessor (only bash can export new env
// though)
// Every command has a timeout, default of 10 minutes.
func (chunk *ExecutableChunk) addCommandToExecute(trimedCommand string, tmpDirs map[string]string) (*RunningCommand, error) {
	var command RunningCommand
	var terminatingError error

	// create the cancel background function, the command.cancelFunc has to get called eventually to avoid leaking
	// memory
	ctx := context.Background()
	ctx, command.cancelFunc = context.WithTimeout(context.Background(), time.Duration(minutesToTimeout)*time.Minute)

	// smart split the command in several pieces
	splited, err := shlex.Split(trimedCommand)
	if err != nil {
		return nil, err
	}
	executable := splited[0]
	args := splited[1:]
	command.cmd = exec.CommandContext(ctx, executable, args...)

	// set the runtime directory for the command
	command.cmd.Dir, terminatingError = chunk.getOrCreateRuntimeDirectory(tmpDirs)
	if terminatingError != nil {
		return nil, terminatingError
	}

	// Copy the environment before calling the command
	command.cmd.Env = append(command.cmd.Env, env...)

	// give a pretty name to the command for the cli output
	command.initCommandLabel(chunk)

	// set the bash flag for the command
	command.isBash = chunk.Runtime == "bash"

	chunk.commands = append(chunk.commands, &command)
	return &command, nil
}

// create a pretty command label to get displayed on the cli. Verbose mode gets more details
func (command *RunningCommand) initCommandLabel(chunk *ExecutableChunk) {
	command.cmdPrettyName = strings.Join(command.cmd.Args, " ")
	if chunk.Label != "" {
		command.cmdPrettyName = chunk.Label
	}
	if verbose {
		command.cmdPrettyName = fmt.Sprint(command.cmdPrettyName, " in ", command.cmd.Dir)
		if len(command.env) > 0 {
			command.cmdPrettyName = fmt.Sprint(command.cmdPrettyName, " with env ", command.env)
		}
	}
}

// start the exec.Cmd instance, in interactive mode, prompts the user to ask what to do
func (command *RunningCommand) start() error {
	if interactive {
		result, _ := pterm.DefaultInteractiveContinue.WithDefaultText(command.cmdPrettyName).Show()
		if result == "all" {
			interactive = false
		}
		if result == "no" {
			command.cmd = nil
			return nil
		}
		if result == "cancel" {
			return errors.New("User aborted")
		}
	}

	command.cmd.Stdout = &command.outb
	command.cmd.Stderr = &command.errb
	if dryRun {
		return nil
	}
	err := command.cmd.Start()
	if err != nil {
		pterm.Error.Printf("%s: %s\n", command.cmdPrettyName, err)
	}
	return err
}

// wait for the command to finish (or the timeout to occur), when the command is bash script, parse the env for the
// next chunk and curate the stdout for the final printing
func (command *RunningCommand) wait() error {
	// don't wait if we're in dryRun mode
	if !dryRun {
		defer command.cancelFunc()
	}
	var spiner *pterm.SpinnerPrinter
	if interactive {
		spiner, _ = pterm.DefaultSpinner.Start("executing")
	} else {
		spiner, _ = pterm.DefaultSpinner.Start(command.cmdPrettyName)
	}
	// handle the interactive scenario where the command doesn't exist because the user skipped it
	if command.cmd == nil {
		spiner.InfoPrinter = &pterm.PrefixPrinter{
			MessageStyle: &pterm.Style{pterm.FgLightBlue},
			Prefix: pterm.Prefix{
				Style: &pterm.Style{pterm.FgBlack, pterm.BgLightBlue},
				Text:  " SKIPPED ",
			},
		}
		spiner.Warning(command.cmdPrettyName)
		return nil
	}
	// wait for the termination
	terminatingError := command.cmd.Wait()
	command.stdout = command.outb.String()
	command.stderr = command.errb.String()

	// handle the output depending on the status of the command
	if terminatingError != nil {
		spiner.Fail("stdout:\n", command.outb.String(), "\nstderr:\n", command.errb.String(), "\nexit code:", command.cmd.ProcessState.ExitCode())
		return terminatingError
	}
	spiner.Success(command.cmdPrettyName)

	// During a bash runtime the user might want to export new variables.
	// Our job here is to recover them to build the new environment for the next chunk
	if command.isBash {
		stoudtLines := strings.Split(command.stdout, "\n")
		// reinitialize the env
		env = []string{}
		env = append(env, os.Environ()...)
		var newLines []string
		var extractVariables = false
		for _, line := range stoudtLines {
			// the environment gets separated by the output from a special string
			if line == "### ENV ###" {
				extractVariables = true
			}
			// then all of it is variables and can be added to the env
			if extractVariables {
				parts := strings.Split(line, "=")
				if len(parts) > 1 {
					env = append(env, line)
				}
			} else {
				// if not it's output we want to keep for the user
				newLines = append(newLines, line)
			}
		}
		if len(newLines) > 0 {
			command.stdout = strings.Join(newLines, "\n")
			command.stdout = command.stdout + "\n"
		} else {
			command.stdout = ""
		}
	}

	// print more things while in verbose mode
	if verbose {
		if command.stdout != "" {
			pterm.Info.Println(command.stdout)
		}
		if command.stderr != "" {
			pterm.Warning.Println(command.stderr)
		}
	}
	return nil
}

// start and wait the exec.Cmd instance
func (command *RunningCommand) execute() error {
	terminatingError := command.start()
	if terminatingError != nil {
		return terminatingError
	}
	terminatingError = command.wait()
	return terminatingError
}

// Executes every stages, chunks and commands of a tutorial if any
func runTutorial(tutorial string) error {
	var tmpDirs map[string]string = make(map[string]string)
	/* when an error occurs during the execution process, every subsequential commands, chunk, and stages are
	 * ignored. As the exception of ones from the `teardown` stage. This is to ensure cleaning up the environment
	 */
	var terminatingError error

	// first get the stages
	stages := extractStages(tutorial)
	if len(stages) == 0 {
		return nil
	}
	pterm.DefaultSection.Println("Testing " + tutorial)

	for _, chunks := range stages {
		// the user can specify a starting point (meaning all the previous stages are ignored) can be used after
		// something got cancelled out
		if startFrom != "" {
			if chunks[0].Stage != startFrom {
				continue
			} else {
				startFrom = ""
			}
		}
		if verbose {
			pterm.DefaultSection.WithLevel(2).Printf("stage %s with %d chunks\n", chunks[0].Stage, len(chunks))
		}
		/* About parallel chunks:
		*  - The commands of a parallel chunk are executed sequentially
		*  - The last command of the chunk runs in parallel with the other chunks of the same stage
		*  - They are awaited at the end of the stage where their effect is applied (if they have any, like
		*    setting a variable for instance)
		 */
		var towait []*ExecutableChunk
		for _, chunk := range chunks {
			// In case an error occurred while executing a previous chunk ignore all but teardown chunks
			if terminatingError != nil && chunk.Stage != "teardown" {
				continue
			}
			if chunk.HasBreakpoint && !ingoreBreakpoints {
				interactive = true
			}

			// If a chunk has a requirement on another chunk being executed successfully, find it and check its status
			// useful for teardown chunks.
			if chunk.Requires != "" {
				reqStageName := strings.Split(chunk.Requires, "/")[0]
				reqChunkId := strings.Split(chunk.Requires, "/")[1]
				reqChunk := findChunkById(stages, reqStageName, reqChunkId)
				// skip the chunk if its requirement didn't execute well
				if reqChunk == nil || !reqChunk.hasExecutedCorrectly() {
					continue
				}
			}

			// Initialize all the commands to run for the chunk
			var hasCreationError bool = false
			if chunk.Runtime == "writer" {
				// Get the runtime directory for the current chunk
				directory, err := chunk.getOrCreateRuntimeDirectory(tmpDirs)
				if err != nil {
					hasCreationError = true
					terminatingError = err
					continue
				}
				// write the file to disk
				err = chunk.writeFile(directory)
				if err != nil {
					hasCreationError = true
					terminatingError = err
					continue
				}
			} else if chunk.Runtime == "bash" {
				// the only difference with bash is that it only have one command to run, which is its script
				uuid := uuid.New()
				cmd := "./" + uuid.String() + ".sh"
				// first create initialize the command to have its env and directory
				command, err := chunk.addCommandToExecute(cmd, tmpDirs)
				if err != nil {
					hasCreationError = true
					terminatingError = err
					continue
				}
				// write the script to disk
				err = chunk.writeBashScript(command.cmd.Dir, cmd)
				if err != nil {
					hasCreationError = true
					terminatingError = err
					continue
				}
			} else {
				for _, command := range chunk.content {
					_, err := chunk.addCommandToExecute(command, tmpDirs)
					if err != nil {
						terminatingError = err
						hasCreationError = true
						break
					}
				}
			}
			// Skip execution if there were issues initializing the commands
			if hasCreationError {
				continue
			}
			// Then execute them
			for commandIndex, command := range chunk.commands {
				// only the last command of a parallel chunk gets executed in parallel of other chunks
				if chunk.IsParallel && commandIndex == len(chunk.commands)-1 {
					err := command.start()
					if err != nil {
						terminatingError = err
						break
					}
					towait = append(towait, chunk)
				} else {
					err := command.execute()
					if err != nil {
						terminatingError = err
						break
					}
				}
			}
		}

		// Wait for the parallel chunks (if any) to terminate their execution, if there's a problem running on of the
		// parallel commands, kill all the other ones.
		for _, chunk := range towait {
			lastCommand := chunk.commands[len(chunk.commands)-1]
			if terminatingError != nil {
				pterm.Warning.Printf("Killing %s\n", lastCommand.cmdPrettyName)
				lastCommand.cmd.Process.Kill()
			} else {
				err := lastCommand.wait()
				if err != nil {
					// In case a parallel chunk got an error executing, we still need to wait for the other chunks to wrap
					// up their work.
					terminatingError = err
				}
			}
		}
	}

	// After execution, update the output part of the chunks based on the result of their execution
	if updateTutorials && terminatingError == nil {
		terminatingError = updateChunkOutput(tutorial, stages)
		if terminatingError == nil {
			os.Rename(path.Join(tutorials_root, tutorial+".out"), path.Join(tutorials_root, tutorial))
		}
	}

	// cleanup the temporary dirs
	for _, tmpDir := range tmpDirs {
		os.RemoveAll(tmpDir)
	}
	return terminatingError
}

func main() {
	RegisterFailHandler(Fail)
	validExtensions := []string{".md", ".MD", ".Markdown", ".markdown"}

	var tutorial string
	flag.BoolVar(&dryRun, "dry-run", false, "just list what would be executed without doing it")
	flag.BoolVar(&interactive, "interactive", false, "prompt to press enter between each chunk")
	flag.BoolVar(&justList, "list", false, "just list the tutorials found")
	flag.BoolVar(&verbose, "verbose", false, "print more logs")
	flag.BoolVar(&noStyling, "no-styling", false, "disable spiners in cli")
	flag.BoolVar(&quiet, "quiet", false, "disable output")
	flag.IntVar(&minutesToTimeout, "timeout", 10, "the timeout in minutes for every executed command")
	flag.StringVar(&startFrom, "start-from", "", "start from a specific stage name")
	flag.StringVar(&tutorials_root, "tutorials-root", "./docs/tutorials", "where to find the tutorials")
	flag.StringVar(&tutorial, "tutorial", "", "Run only a specific tutorial")
	flag.BoolVar(&ingoreBreakpoints, "ignore-breakpoints", false, "ignore the breakpoints")
	flag.BoolVar(&updateTutorials, "update-tutorials", false, "update the chunk output section in the tutorials")
	flag.Parse()
	tutorials, err := os.ReadDir(tutorials_root)
	Expect(err).To(BeNil())
	if noStyling {
		pterm.DisableStyling()
	}
	if quiet {
		pterm.DisableOutput()
	}
	env = append(env, os.Environ()...)
	workding_directory, err := os.Getwd()
	pterm.Fatal.PrintOnError(err)
	env = append(env, "OPERATOR_ROOT="+workding_directory)

	for _, e := range tutorials {
		// avoid parsing files that aren't markdown
		extension := path.Ext(e.Name())
		if !slices.Contains(validExtensions, extension) {
			continue
		}
		// only list the available files
		if justList {
			pterm.Info.Println(e.Name())
			continue
		}
		// if a single tutorial is selected, only execute this one
		if tutorial != "" {
			if path.Base(tutorial) != e.Name() {
				pterm.Info.Println("Ignoring", e.Name())
				continue
			}
		}
		// parse and execute if possible
		err := runTutorial(e.Name())
		if err != nil {
			panic(err)
		}
	}
}
