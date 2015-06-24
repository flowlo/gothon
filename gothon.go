package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
)

var (
	debug   = flag.Bool("debug", false, "continuously print debug messages")
	version = flag.Bool("version", false, "only print version and build information, exit immediately")
)

var Version string
var BuildTime string

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: \n")
		fmt.Fprintf(os.Stderr, "       %s [flags] <filename>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "        for interpreting Python code in <filename>\n")
		fmt.Fprintf(os.Stderr, "       %s [flags]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "        for running a REPL\n\n")
		fmt.Fprintf(os.Stderr, "Available flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *version {
		printVersion()
		os.Exit(0)
	}

	if len(flag.Args()) == 0 {
		repl()
	}

	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	file, err := resolve(flag.Args()[0])
	defer file.Close()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	module := &Module{}
	reader := Reader{*bufio.NewReader(file), *module}

	module.Read(&reader, 0)

	frame := NewFrame(module.Code)
	frame.Execute()
}

func resolve(target string) (file *os.File, err error) {
	if path.Ext(target) == ".pyc" {
		file, err = os.Open(target)
		return
	}

	if _, err := exec.LookPath("python3.4"); err != nil {
		err = fmt.Errorf("gothon: python3.4 needed for compilation")
		return nil, err
	}

	cmd := exec.Command("python3.4", "-m", "compileall", "-l", target)
	output, err := cmd.CombinedOutput()

	//if len(output) > 0 {
	fmt.Print(string(output))
	//}

	if err != nil {
		return
	}

	base := path.Base(target)
	target = path.Join(path.Dir(target), "__pycache__", base[:len(base)-2]+"cpython-34.pyc")
	file, err = os.Open(target)
	return
}

// Takes some Python source code and compiles it by passing it
// to an external Python compiler.
func compile(code string) (output []byte, err error) {
	inject := `
import marshal
print(marshal.dumps(compile("` + code + `", "<repl>", "exec")))
	`

	cmd := exec.Command("python3.4")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		return
	}

	stdin.Write([]byte(inject))
	stdin.Close()

	output, err = ioutil.ReadAll(stdout)
	if err != nil {
		return
	}
	stdout.Close()

	err = cmd.Wait()

	if err != nil {
		return
	}
	s := "\"" + string(output[2:len(output)-2]) + "\""
	t, err := strconv.Unquote(s)
	if err != nil {
		return
	}
	output = []byte(t)
	return
}

// Loops over user input until EOF on standard input.
func repl() {
	printVersion()
	fmt.Println("Type \"copyright\" for more information.")

	for {
		print(">>> ")
		bio := bufio.NewReader(os.Stdin)
		raw, _, err := bio.ReadLine()
		if err != nil {
			if err == io.EOF {
				fmt.Println()
				os.Exit(0)
			}
			panic(err)
		}

		input := string(raw)

		if input == "copyright" {
			fmt.Println("Copyright (c) 2015 Lorenz Leutgeb.")
			continue
		}

		raw, err = compile(input)
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				continue
			}
			panic(err)
		}

		module := &Module{}
		reader := Reader{*bufio.NewReader(bytes.NewReader(raw)), *module}
		code := reader.ReadCode()

		frame := NewFrame(code)
		frame.Execute()
	}
}

// Prints version information to standard output.
func printVersion() {
	fmt.Println("gothon " + Version + " (" + BuildTime + ")")
	fmt.Println("[" + runtime.Version() + "]")
}
