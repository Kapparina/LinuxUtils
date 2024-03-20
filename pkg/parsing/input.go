package parsing

import (
	"bufio"
	"flag"
	"os"
	"strings"
)

func GetInput() (filePath string, inputErr error) {
	flag.Parse()
	if arg := flag.Arg(0); len(arg) > 1 {
		filePath = arg
	}
	if len(filePath) > 0 {
		filePath = strings.TrimSpace(filePath)
	} else {
		pipeInput, pipeErr := os.Stdin.Stat()
		if pipeErr != nil {
			inputErr = pipeErr
		}
		if pipeInput.Mode()&os.ModeNamedPipe != 0 {
			reader := bufio.NewReader(os.Stdin)
			input, bufferErr := reader.ReadString('\n')
			if bufferErr != nil {
				inputErr = bufferErr
			} else {
				filePath = strings.TrimSpace(input)
			}
		}
		if len(filePath) < 1 {
			cwd, err := os.Getwd()
			if err != nil {
				inputErr = err
			} else {
				filePath = cwd
			}
		}
	}
	return
}
