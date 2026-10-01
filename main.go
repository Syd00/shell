package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type BuiltinFunc func(args []string, stdOut io.Writer)

var builtins map[string]BuiltinFunc

func init() {
	builtins = map[string]BuiltinFunc{
		"exit": handleExit,
		"echo": handleEcho,
		"type": handleType,
		"pwd":  handlePwd,
		"cd":   handleCd,
	}
}

// Parse the line (CLI command) and split the command part and the args part
func parse(line string) (string, []string, error) {

	singleQuotes := false
	doubleQuotes := false
	escaped := false
	hasToken := false

	var tokens []string
	var current strings.Builder

	line = strings.TrimRight(line, "\r\n")
	runes := []rune(line)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		if escaped {
			current.WriteRune(ch)
			escaped = false
			hasToken = true
			continue
		}

		if ch == '\\' {
			if singleQuotes {
				current.WriteRune(ch)
				hasToken = true
			} else if doubleQuotes {
				if i+1 < len(line) && (runes[i+1] == '\\' || runes[i+1] == '$' || runes[i+1] == '"' || runes[i+1] == '\n') {
					escaped = true
				} else {
					current.WriteRune(ch)
					hasToken = true
				}
			} else {
				escaped = true
			}
			continue
		}

		if ch == '"' && !singleQuotes {
			doubleQuotes = !doubleQuotes
			hasToken = true
			continue

		}

		if ch == '\'' && !doubleQuotes {
			singleQuotes = !singleQuotes
			hasToken = true
			continue
		}

		if (ch == ' ' || ch == '\n') && !singleQuotes && !doubleQuotes {
			if hasToken {
				tokens = append(tokens, current.String())
				current.Reset()
				hasToken = false
			}
			continue
		}
		current.WriteRune(ch)
		hasToken = true
	}

	if hasToken {
		tokens = append(tokens, current.String())
	}

	if len(tokens) == 0 {
		return "", nil, nil
	}

	return tokens[0], tokens[1:], nil
}

// Check command type and exec with args
func eval(cmd string, args []string) {
	var stdOut io.Writer = os.Stdout
	var outFile *os.File

	index := slices.IndexFunc(args, func(arg string) bool {
		return arg == ">" || arg == "1>"
	})

	if index != -1 {
		var err error
		targetPath := args[index+1]

		// Ricava la directory padre (nel tuo caso "/tmp/fox") e creala se non esiste
		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return
		}
		outFile, err = os.Create(args[index+1])
		if err != nil {
			fmt.Print("Error during creating file")
			return
		}
		defer outFile.Close()
		stdOut = outFile
		args = append(args[:index], args[index+2:]...)
	}

	// Check if builtin command and exec
	if handler, ok := builtins[cmd]; ok {
		handler(args, stdOut)
		return
	}

	// Check if exe in PATH
	if _, err := exec.LookPath(cmd); err == nil {
		handleExe(cmd, args, stdOut)
		return
	}

	fmt.Printf("%s: command not found\n", cmd)
}

func handleExit(args []string, stdOut io.Writer) {
	os.Exit(0)
}

func handleEcho(args []string, stdOut io.Writer) {
	fmt.Fprintln(stdOut, strings.Join(args, " "))
}

func handleType(args []string, stdOut io.Writer) {
	if len(args) == 0 {
		return
	}

	target := args[0]

	if _, ok := builtins[target]; ok {
		fmt.Println(target + " is a shell builtin")
	} else {
		path, err := exec.LookPath(strings.Split(target, " ")[0])
		if err == nil {
			fmt.Printf("%s is %s\n", target, path)
		} else {
			fmt.Printf("%s: not found\n", target)
		}
	}
}

func handlePwd(args []string, stdOut io.Writer) {
	dir, err := os.Getwd()

	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}

	fmt.Printf("%s\n", dir)
}

func handleCd(args []string, stdOut io.Writer) {
	var targetDir string

	if len(args) == 0 || args[0] == "~" {
		home, err := os.UserHomeDir()

		if err != nil {
			fmt.Printf("Unable to resolve home directory")
		}
		targetDir = home
	} else {
		targetDir = strings.Join(args, " ")
	}

	if err := os.Chdir(targetDir); err != nil {
		fmt.Printf("cd: %s: No such file or directory\n", targetDir)
	}
}

func handleExe(name string, args []string, stdOut io.Writer) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdOut
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error during execution\n")
		return
	}
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("$ ")

		// Wait for the user input
		command, err := reader.ReadString('\n')

		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input", err)
			os.Exit(1)
		}

		cmd, args, err := parse(command)
		if err != nil {
			fmt.Printf("Parse error: %v\n", err)
			continue
		}

		if cmd == "" {
			continue
		}

		eval(cmd, args)
	}
}
