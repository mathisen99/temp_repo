package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// PythonDockerTool embeds BaseTool and implements the Tool interface.
type PythonDockerTool struct {
	BaseTool
}

// NewPythonDockerTool creates and returns a pointer to a new PythonDockerTool.
func NewPythonDockerTool() *PythonDockerTool {
	return &PythonDockerTool{
		BaseTool: BaseTool{
			ToolName:        "runPythonCode",
			ToolDescription: "Execute Python code in a secure Docker container and return the output. Python 3.13 environment includes numpy, pandas, matplotlib, scipy, sympy, scikit-learn, and pillow libraries for data science and math operations.",
			ToolParameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"code": {
						Type:        jsonschema.String,
						Description: "Python code to be executed. Must include print statements to produce output. Limited to 1 minute execution time, 128MB memory, and no network access. INCLUDE COMPLETE CODE: When asked to create a program, submit fully executable code, not fragments. Example: A proper earth spin calculator would define a function, calculate the value (360/24=15 degrees per hour), and print the result.",
					},
				},
				Required: []string{"code"},
			},
		},
	}
}

// Execute runs the Python code in Docker and returns the output or an error.
func (p *PythonDockerTool) Execute(args string) (string, error) {
	var params struct {
		Code string `json:"code"`
	}

	if err := json.Unmarshal([]byte(args), &params); err != nil {
		logger.Errorf("Invalid arguments for runPythonCode: %v", err)
		return "", fmt.Errorf("invalid arguments: %v", err)
	}
	if params.Code == "" {
		return "", fmt.Errorf("code cannot be empty")
	}

	output, err := runPythonInDocker(params.Code)
	if err != nil {
		logger.Errorf("Failed to execute Python code: %v", err)
		// Return the error message as part of the output rather than failing
		return fmt.Sprintf(
			"Executed Python Code:\n```\n%s\n```\n\nError:\n```\n%v\n```",
			params.Code, err,
		), nil
	}

	return fmt.Sprintf(
		"Executed Python Code:\n```\n%s\n```\n\nOutput:\n```\n%s\n```",
		params.Code, output,
	), nil
}

// runPythonInDocker writes code to a temp file, mounts it into a Docker container,
// executes it there, and captures the output.
func runPythonInDocker(code string) (string, error) {
	// Clean up old script files that might be left from previous executions
	cleanupOldTempFiles()
	
	if err := os.MkdirAll("tmp", 0o755); err != nil {
		return "", fmt.Errorf("failed to create tmp directory: %v", err)
	}

	tmpFile, err := os.CreateTemp("tmp", "script-*.py")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err = tmpFile.WriteString(code); err != nil {
		return "", fmt.Errorf("failed to write code: %v", err)
	}
	if err = tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file: %v", err)
	}

	// Make sure script is executable (just in case).
	if err = os.Chmod(tmpFile.Name(), 0o755); err != nil {
		return "", fmt.Errorf("failed to set file permissions: %v", err)
	}

	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	// We'll give the Docker command 1 minute to complete.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// We specify -w /tmp to force the working directory to /tmp inside the container,
	// then pass script.py as an argument. Also switch to python3 if needed.
	cmd := exec.CommandContext(
		ctx,
		"docker",
		"run",
		"--rm",
		"--memory=128m",
		"--cpus=0.5",
		"--user=nonrootuser",
		"--security-opt=no-new-privileges:true",
		"--cap-drop=ALL",
		"--network=none",
		"-v", fmt.Sprintf("%s:/tmp/script.py", absPath),
		"-w", "/tmp",
		"python-math-libs",
		"python",
		"script.py",
	)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "Execution exceeded the 1-minute time limit.", nil
	}
	if err != nil {
		// Check if the error is due to a timeout that was caught by Docker instead of our context
		if string(output) == "" {
			return "", fmt.Errorf("execution failed with no output (possible timeout)")
		}
		// Return a more descriptive error that includes the command output
		return "", fmt.Errorf("execution failed: %v\nOutput: %s", err, output)
	}

	// Trim output if it's too long
	outStr := string(output)
	const maxOutputLength = 4000
	if len(outStr) > maxOutputLength {
		return outStr[:maxOutputLength] + "\n... [output truncated due to length]", nil
	}

	return outStr, nil
}

// cleanupOldTempFiles removes Python script files in the tmp directory
// that are older than 1 hour to prevent clutter
func cleanupOldTempFiles() {
	tmpDir := "tmp"
	files, err := filepath.Glob(filepath.Join(tmpDir, "script-*.py"))
	if err != nil {
		logger.Errorf("Failed to list temp files: %v", err)
		return
	}

	cutoffTime := time.Now().Add(-1 * time.Hour)
	for _, file := range files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			continue
		}
		if fileInfo.ModTime().Before(cutoffTime) {
			if err := os.Remove(file); err != nil {
				logger.Errorf("Failed to remove old temp file %s: %v", file, err)
			}
		}
	}
}