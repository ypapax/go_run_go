package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	maximumLimitOfStoreInMegabytes        = 100         // megabytes
	deleteFilesAndFoldersOlderThanMinutes = 60 * 24 * 2 // 2 days
	maximumCleanCacheAttempts             = 20          // in order to prevent infinite loop
	// if new go code is not inputed for this period of time
	// then the program will check and output new results from previous
	// go programs
	waitForInputMilliseconds = 20000
)

/*
starting point of the program
*/
func main() {
	p("Welcome to runner of Go code as a string with time measuring")
	cleanCache()

	// channel for console input
	codeOrExpectedTimeInputChan := make(chan string, 1)
	go readFromConsoleAndReadFromFileIfNeed(codeOrExpectedTimeInputChan)
	for {
		printNewResults()
		askToInputCode()
		/*
			When a user ends up with inputting text to console
			he have to press ^] and then "Enter".

			It's because  we let a user input multiline text, so we have to choose any delemiter
			to stop inputting, except '\n' (pressing Enter)
		*/
		var code string
		select {
		case code = <-codeOrExpectedTimeInputChan:
			p("code")
			p(code)
			/*
				if we can get the Go code, then ask to input expected time for execution in milliseconds
			*/
			if len(code) > 0 {
				p("Please input maximum expected time in milliseconds for execution this code, use CTRL+] to end input")
				expectedMillis := stringToFloat64(<-codeOrExpectedTimeInputChan)
				doAllByGoCodeAndCompareWithExpectation(code, expectedMillis)
			}
		case <-time.After(waitForInputMilliseconds * time.Millisecond):
		}

	}
	return
}

/*
after converting string value of expected milliseconds
we need to convert it to a number to compare with actual time
*/
func stringToFloat64(str string) (f float64) {
	var err error
	f, err = strconv.ParseFloat(str, 64)
	check(err)
	return
}

/*
run the second Go code and compare time result with an expected value
and outputs result to the terminal
*/
func doAllByGoCodeAndCompareWithExpectation(goCode string, expectedMillis float64) (millisResult float64, okTime, okResult bool, stderr, stdout string) {
	millisResult, okTime, okResult, stderr, stdout = doAllByGoCode(goCode, expectedMillis)
	return
}

/*
run Go code and compare with expected time
if it is the first time to run this code, it will save it do disk, compile and only then run
*/
func doAllByGoCode(goCode string, expectedMillis float64) (millisResult float64, okTime, okResult bool, stderr, stdout string) {
	p("We have the following go code")
	p(goCode)
	var (
		currentGoFileExecutable  string
		preparationToRunningIsOk bool
	)
	_, _, currentGoFileExecutable, preparationToRunningIsOk = writeGoFileToDiskAndBuildAndGenerateFileName(goCode)
	// here must be run as async, because it is the most long term part
	if preparationToRunningIsOk {
		p("running " + currentGoFileExecutable)
		// do not wait for finishing this go code
		// go further
		go runGoCodeAndMeasureTimeAndSaveInResultList(currentGoFileExecutable, expectedMillis)
	}
	return
}

/*
run executable by full path (compiled go code) and returns ok if no erros
and returns actual wasted milliseconds for it.
*/
func runGoCodeAndMeasureTime(executablePath string, expectedMs float64) (millisResult float64, okTime, okResult bool, stderr, stdout string) {
	okTime, okResult, stdout, stderr, millisResult = measureTime(func() (bool, string, string) { return runGoCode(executablePath, time.Duration(expectedMs)) }, expectedMs)
	return
}

/*
run executable by full path (compiled go code) and returns ok if no erros
and returns actual wasted milliseconds for it.
*/
func runGoCodeAndMeasureTimeAndSaveInResultList(executablePath string, expectedMs float64) {
	millisResult, okTime, okResult, stderr, stdout := runGoCodeAndMeasureTime(executablePath, expectedMs)
	newResult := &Result{
		OkTime:         okTime,
		OkResult:       okResult,
		StdErr:         stderr,
		StdOut:         stdout,
		ActualMillis:   millisResult,
		Name:           executablePath,
		ExpectedMillis: expectedMs,
	}
	readyResults = append(readyResults, newResult)
	return
}

func runGoCode(executablePath string, expectedMillis time.Duration) (okNoErrors bool, stdout, stderr string) {
	okNoErrors, stdout, stderr = runBashCommandAndKillIfTooSlow(executablePath, expectedMillis)
	return
}

/*
write go code by a string to a special dir ~/SecondGoStore/hashByThisGoCode
*/
func writeGoFileToDiskAndBuildAndGenerateFileName(goCodeStr string) (folderForStoringCurrentGoFile, currentGoFileFullPath, currentGoFileExecutable string, okWithoutErrors bool) {
	currentGoJustFileName := getHash(goCodeStr)

	folderForStoringCurrentGoFile, currentGoFileFullPath, currentGoFileExecutable, okWithoutErrors = writeGoFileToDiskAndBuild(currentGoJustFileName, goCodeStr)
	return
}

/*
write go code by a string to disk, and compile, and return pathes
in order to run this go code later
*/
func writeGoFileToDiskAndBuild(currentGoJustFileName, goCodeStr string) (folderForStoringCurrentGoFile, currentGoFileFullPath, currentGoFileExecutable string, okWithoutErrors bool) {
	var (
		folderForStoringAllGoFiles string = generateFolderForStoringSecondGoFiles()
	)
	folderForStoringCurrentGoFile, currentGoFileFullPath, currentGoFileExecutable = getPathes(folderForStoringAllGoFiles, currentGoJustFileName)
	if !exists(currentGoFileExecutable) {
		p("This Go code had never been run before, so we need to write it to disk and compile")
		/*
			create a folder where we place the second Go code
		*/
		if _, ok := bashCommand("mkdir -p " + folderForStoringCurrentGoFile); !ok {
			return
		}

		if exists(folderForStoringCurrentGoFile) {
			/*
				writing Go file to disk
			*/
			strToFile(goCodeStr, currentGoFileFullPath)
			/*
				compiling Go file
			*/
			command := fmt.Sprintf("cd %s && go build", folderForStoringCurrentGoFile)
			if _, ok := bashCommand(command); !ok {
				return
			}
		} else {
			p(folderForStoringCurrentGoFile + " do not exist, so cannot run Go code")
		}
	} else {
		/*
			For speed up purpose do not create the same folder again.
			If the folder with the name exists,
			then it means a user is trying to execute the same code again
			so no need to mkdir, write file, go build, again

			just return pathes by coresponding to currentGoJustFileName
		*/
		p("This Go code had been run before, so no need to write to disk and compile")
	}
	okWithoutErrors = true
	return
}

/*
place to store the second Go code and executables
*/
const commonFolderName = "SecondGoStore"

/*
absolute path for storing all the Second Go code
*/
func generateFolderForStoringSecondGoFiles() (secondGoStoreFolderForAllGoFiles string) {
	secondGoStoreFolderForAllGoFiles = UserHomeDir() + "/" + commonFolderName
	return
}

/*
generate all pathes by Go filename (hashes)
*/
func getPathes(folderForStoringAllGoFiles, currentGoJustFileName string) (folderForStoringCurrentGoFile, currentGoFileFullPath, currentGoFileExecutable string) {
	folderForStoringCurrentGoFile = folderForStoringAllGoFiles + "/" + currentGoJustFileName
	currentGoFileExecutable = folderForStoringCurrentGoFile + "/" + currentGoJustFileName
	currentGoFileFullPath = currentGoFileExecutable + ".go"
	return
}

/*
prints error if it occurs
*/
func check(err error) (okWithoutErrors bool) {
	if err != nil {
		fmt.Println("Error")
		fmt.Println(err)

	} else {
		okWithoutErrors = true
	}
	return
}

/*
save to file
*/
func strToFile(str, filename string) {
	p("Writing file " + filename)
	f, err := os.Create(filename)
	check(err)
	_, err2 := f.WriteString(str)
	check(err2)
}

/*
run bash command and return result
*/
func bashCommand(command string) (output string, okWithoutErrors bool) {
	p("executing bash command...")
	p(command)
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	okWithoutErrors = check(err)
	return
}

/*
run bash command and kill it if it works longer than "killInMilliSeconds" milliseconds
*/
func runBashCommandAndKillIfTooSlow(command string, killInMilliSeconds time.Duration) (okResult bool, stdout, stderr string) {
	// p("running bash command...")
	// p(command)
	cmd := exec.Command("sh", "-c", command)

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	okResult = true

	err := cmd.Start()
	// log.Printf("Waiting for command to finish...")
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(killInMilliSeconds * time.Millisecond):
		if err := cmd.Process.Kill(); err != nil {
			log.Fatal("failed to kill: ", err)
			okResult = false
		}
		<-done // allow goroutine to exit
		// log.Println("process killed")
	case err := <-done:
		if err != nil {
			// log.Printf("process done with error = %v", err)
			okResult = false
		}
		stdout = outbuf.String()
		stderr = errbuf.String()
	}
	if err != nil {
		log.Fatal(err)
		okResult = false
	}
	return
}

/*
milliseconds differense between two points in code
*/
func calculateTimeDiff(timeStart time.Time, msg string) (actualMs float64, now time.Time) {

	now = time.Now()
	diff := now.Sub(timeStart)
	actualMs = diff.Seconds() * 1000
	return
}

/*
let a user input line to console and read it to string variable; and return it.
*/
func readFromConsole(msg string) (result string) {
	result = readFromConsoleByDelemiter(msg, '\n')
	return
}

/*
read from console until delemiter
*/
func readFromConsoleByDelemiter(msg string, delemiter byte) (result string) {
	in := bufio.NewReader(os.Stdin)
	text, err := in.ReadString(delemiter)
	if err != nil {
		fmt.Println(err)
	}
	/*
		cutting of last char - it's a delemiter
	*/
	result = text[:len(text)-1]
	result = trim(result)
	return
}

/*
Print to console message to ask a user input Go code
*/
func askToInputCode() {
	p("Please input Go code (less than 1000 chars) or /absolute/path/to/Go/file.go of any size. To end input please press CTRL+] and then Enter")
}

/*
for reading multilines input from a user
*/
var scn = bufio.NewScanner(os.Stdin)

/*
read multiline console input

to end input just press ^] and then enter
^] means ctrl-]
*/
func readFromConsoleMultilines(msg string) (result string) {
	result = readFromConsoleByDelemiter(msg, '\x1D')
	return
}

/*
returns true if inputtedQueryOrFilepath is absolute path to the file
*/
func detectFileName(inputtedQueryOrFilepath string) (yesItIsFileName bool) {
	yesItIsFileName = strings.HasPrefix(inputtedQueryOrFilepath, "/")
	return
}

/*
if a user inputted an absolute file name, then it reads the file and return as a string.
If it's not a filename then just return inputted text
*/
func readFromConsoleAndReadFromFileIfNeed(input chan string) {
	for {
		humanInputed := readFromConsoleMultilines("")
		itsAfileName := detectFileName(humanInputed)
		var (
			result string
		)
		if itsAfileName {
			p(fmt.Sprintf("Reading from file %s", humanInputed))
			result, _ = fileToStr(humanInputed)
		} else {
			result = humanInputed
		}
		input <- result
	}
}

/*
get file content by its path
*/
func fileToStr(path string) (fileContent string, ok bool) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	ok = true
	fileContent = string(bytes)
	return
}

/*
print to console with new line
*/
func p(obj interface{}) {
	fmt.Println(obj)
}

/*
remove head and tail spaces of a string
*/
func trim(s string) string {
	return strings.TrimSpace(s)
}

// exists returns whether the given file or directory exists or not
func exists(path string) bool {
	_, err := os.Stat(path)

	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	} else {
		check(err)
	}
	return true
}

/*
get home directory of an OS user
*/
func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

/*
converts any string to a hash like b1a39a26ea62a5c075cd3cb5aa46492c8e1134b7
*/
func getHash(str string) (hash string) {
	// The pattern for generating a hash is `sha1.New()`,
	// `sha1.Write(bytes)`, then `sha1.Sum([]byte{})`.
	// Here we start with a new hash.
	h := sha1.New()

	// `Write` expects bytes. If you have a string `str`,
	// use `[]byte(s)` to coerce it to bytes.
	h.Write([]byte(str))

	// This gets the finalized hash result as a byte
	// slice. The argument to `Sum` can be used to append
	// to an existing byte slice: it usually isn't needed.
	hash = fmt.Sprintf("%x", h.Sum(nil))

	// SHA1 values are often printed in hex
	return
}

/*
type used for measuring time of a function
*/
type measuredFunc func() (bool, string, string)

// measures execution time of a function f
// if expectedMs milliseconds passed and f
// still did not finish execution it will return
// without waiting for f
func measureTime(f measuredFunc, expectedMs float64) (okTime, okResult bool, stderr, stdout string, actualMs float64) {
	done := make(chan bool)
	t1 := time.Now()

	go func() {
		okResult, stderr, stdout = f()
		close(done)
	}()

	select {
	case <-done:
		okTime = true
	case <-time.After(time.Duration(expectedMs) * time.Millisecond):
	}
	actualMs = time.Since(t1).Seconds() * 1000
	return
}

/*
run bash command and return text result as a string
*/
func runBashCommandAndGetResult(command string) (result string) {
	fmt.Println("running command")
	fmt.Println(command)
	output, e := exec.Command("sh", "-c", command).Output()
	if e != nil {
		fmt.Println("error")
		fmt.Println(e)
	}
	result = trim(string(output))
	p(result)
	return
}

/*
How many megabytes in a folder
*/
func getFolderSizeInMegabytes(folder string) (megabytes int) {
	megaBytesStr := runBashCommandAndGetResult(fmt.Sprintf("du -cshm %s | grep total  | awk '{print $1}'", folder))
	megabytes = strToInt(megaBytesStr)
	return
}

/*
delete all in a "folder" older than "hours" hours
*/
func deleteOlderThan(folder string, hours int) {
	p(fmt.Sprintf("Deleting files in %s older than %f hours", folder, float64(hours)/60))
	bashCommand(fmt.Sprintf("find %s -mmin +%d -delete", folder, hours))
	return
}

/*
convert string to integer
*/
func strToInt(s string) (i int) {
	var err error
	i, err = strconv.Atoi(s)
	if err != nil {
		// handle error
		fmt.Println(err)
	}
	return
}

/*
if cache size is bigger than limit
delete all files in cache older than "deleteFilesAndFoldersOlderThanMinutes" hours
*/
func cleanCache() {
	targetFolder := generateFolderForStoringSecondGoFiles()
	if tooHeavy, _ := cacheIsTooHeavy(maximumLimitOfStoreInMegabytes); tooHeavy {
		/*
			attempt 1 to clean cache
			delete files and folders older than "deleteFilesAndFoldersOlderThanMinutes" minutes
		*/
		deleteOlderThan(targetFolder, deleteFilesAndFoldersOlderThanMinutes)

		if tooHeavy, _ := cacheIsTooHeavy(maximumLimitOfStoreInMegabytes); tooHeavy {
			/*
				attempt #2 to clean cache
				delete the oldest folders one by one until cache will be
				less than "maximumLimitOfStoreInMegabytes"
			*/
			removeOldestDirsInCacheUntilCacheSizeBecomesLessThan(maximumLimitOfStoreInMegabytes)
		}
	}
}

/*
find and delete the oldest dir in the cache
*/
func removeOldestDirInCache() {
	targetFolder := generateFolderForStoringSecondGoFiles()
	p(fmt.Sprintf("Removing the oldest dir in the directory %s", targetFolder))
	bashCommand(fmt.Sprintf(`cd %s && rm -R $(ls -lt %s | grep '^d' | tail -1  | tr " " "\n" | tail -1)`, targetFolder, targetFolder))
}

/*
delete the oldest dir in the cache
check the cache size
if cache is not ok
delete one more oldest dir in the cache

and so on until cache will become less than the limit in megabytes
*/
func removeOldestDirsInCacheUntilCacheSizeBecomesLessThan(maximumCacheSizeInMegabytes int) {
	for i := 0; i < maximumCleanCacheAttempts; i++ {
		removeOldestDirInCache()
		if tooHeavy, _ := cacheIsTooHeavy(maximumCacheSizeInMegabytes); !tooHeavy {
			break
		}
	}
	/*
		if cache is still big
	*/
	if tooHeavy, _ := cacheIsTooHeavy(maximumCacheSizeInMegabytes); tooHeavy {
		/*
			remove all
		*/
		removeAllTheCache()
		p("Finally")
		cacheIsTooHeavy(maximumCacheSizeInMegabytes)
	}
}

/*
check size of the cache
*/
func cacheIsTooHeavy(maximumSizeInMB int) (tooHeavy bool, actualStoreMegaBytes int) {
	targetFolder := generateFolderForStoringSecondGoFiles()
	p("Checking cache size in megabytes")
	actualStoreMegaBytes = getFolderSizeInMegabytes(targetFolder)
	tooHeavy = actualStoreMegaBytes >= maximumSizeInMB
	if tooHeavy {
		p(fmt.Sprintf("Store size %d megabytes is larger than limit of %d megabytes", actualStoreMegaBytes, maximumSizeInMB))
	} else {
		p(fmt.Sprintf("Ok. Cache size (%s) is %d megabyts. It is not bigger than limit %d megabytes", targetFolder, actualStoreMegaBytes, maximumSizeInMB))
	}
	return
}

/*
remove all from cache
*/
func removeAllTheCache() {
	targetFolder := generateFolderForStoringSecondGoFiles()
	p(fmt.Sprintf("Removing all in %s", targetFolder))
	bashCommand(fmt.Sprintf(`cd %s && rm -r %s/*`, targetFolder, targetFolder))
}

/*
result type
*/
type Result struct {
	StdErr, StdOut               string
	OkTime, OkResult             bool
	ActualMillis, ExpectedMillis float64
	Name                         string
}

/*
type of list of results
*/
type ResultList []*Result

/*
results from previous go programs
*/
var readyResults ResultList

/*
check if there are new results and prints them
after printing it cleans all new results
*/
func printNewResults() {
	if len(readyResults) > 0 {
		p("We have new results")
		for _, r := range readyResults {
			p("Stdout for " + r.Name)
			p(r.StdOut)
			p("Results for " + r.Name)
			okResult := r.OkResult
			okTime := r.OkTime
			if okTime && !okResult {
				p("Error occured while executing the Second Go code")
				p(r.StdErr)
			} else if !okTime {
				p(fmt.Sprintf("%f ms. Fail. It's slower than you expected: %f ms", r.ActualMillis, r.ExpectedMillis))
			} else if okTime {
				p(fmt.Sprintf("%f ms. Ok. It's faster than you expected: %f ms", r.ActualMillis, r.ExpectedMillis))
			}
		}
		readyResults = readyResults[:0]
	} else {
		p("We have no new results")
	}

}
