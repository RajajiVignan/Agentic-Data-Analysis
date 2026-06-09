package api

import (
	"fmt"
	"regexp"
	"strings"
)

// SandboxValidator checks LLM-generated Python code for dangerous patterns
// before execution. It is a static analysis pass — no code is executed.
type SandboxValidator struct {
	// blockedImports are modules that must not appear in import statements.
	blockedImports []string
	// blockedPatterns are regex patterns that must not appear anywhere in the code.
	blockedPatterns []*regexp.Regexp
	// blockedPatternDescriptions human-readable descriptions for blockedPatterns.
	blockedPatternDescriptions []string
	// maxCodeLen is the maximum allowed code size in bytes.
	maxCodeLen int
}

// NewSandboxValidator creates a validator with sensible defaults.
func NewSandboxValidator() *SandboxValidator {
	patterns := []string{
		// Dangerous builtins
		`\beval\s*\(`,
		`\bexec\s*\(`,
		`\bcompile\s*\(`,
		`\bglobals\s*\(`,
		`\blocals\s*\(`,
		`\bgetattr\s*\(`,
		`\bsetattr\s*\(`,
		`\bdelattr\s*\(`,
		// File system writes outside of plt.savefig
		`\bopen\s*\([^)]*['\"][wa]`,
		// Subprocess / system calls
		`\bos\.system\s*\(`,
		`\bos\.popen\s*\(`,
		`\bos\.exec[lv]`,
		`\bos\.spawn`,
		`\bos\.fork\s*\(`,
		`\bos\.kill\s*\(`,
		`\bos\.remove\s*\(`,
		`\bos\.unlink\s*\(`,
		`\bos\.rmdir\s*\(`,
		`\bos\.rename\s*\(`,
		`\bos\.chmod\s*\(`,
		`\bos\.chown\s*\(`,
		// Subprocess module
		`\bsubprocess\.`,
		// Network access
		`\bsocket\.`,
		`\burllib`,
		`\bhttplib`,
		`\bhttp\.client`,
		`\brequests\.`,
		// Code injection via import hooks
		`\b__import__\s*\(`,
		`\bimportlib`,
		// Dangerous dunder access
		`\b__builtins__`,
		`\b__class__`,
		`\b__subclasses__`,
		`\b__globals__`,
		// Pickle / serialization attacks
		`\bpickle\.`,
		`\bmarshal\.`,
		`\byaml\.load\b`,
		// Shell injection
		`\bshutil\.`,
		// ctypes / ffi
		`\bctypes\.`,
		`\bcffi\.`,
		// Multiprocessing / threading escapes
		`\bmultiprocessing\.`,
		`\bthreading\.`,
	}

	descriptions := []string{
		"eval() call",
		"exec() call",
		"compile() call",
		"globals() call",
		"locals() call",
		"getattr() call",
		"setattr() call",
		"delattr() call",
		"file write via open()",
		"os.system() call",
		"os.popen() call",
		"os.exec*() call",
		"os.spawn*() call",
		"os.fork() call",
		"os.kill() call",
		"os.remove() call",
		"os.unlink() call",
		"os.rmdir() call",
		"os.rename() call",
		"os.chmod() call",
		"os.chown() call",
		"subprocess module",
		"socket module",
		"urllib module",
		"httplib module",
		"http.client module",
		"requests module",
		"__import__() call",
		"importlib module",
		"__builtins__ access",
		"__class__ access",
		"__subclasses__ access",
		"__globals__ access",
		"pickle module",
		"marshal module",
		"yaml.load() call",
		"shutil module",
		"ctypes module",
		"cffi module",
		"multiprocessing module",
		"threading module",
	}

	compiled := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		compiled[i] = regexp.MustCompile(p)
	}

	return &SandboxValidator{
		blockedImports: []string{
			"os", "subprocess", "socket", "urllib", "httplib",
			"http.client", "requests", "importlib", "pickle",
			"marshal", "shutil", "ctypes", "cffi",
			"multiprocessing", "threading", "code", "compile",
			"pty", "signal", "platform",
		},
		blockedPatterns:            compiled,
		blockedPatternDescriptions: descriptions,
		maxCodeLen:                 50000,
	}
}

// ValidationResult holds the outcome of a sandbox validation.
type ValidationResult struct {
	OK         bool     `json:"ok"`
	Violations []string `json:"violations,omitempty"`
}

// Validate checks the given Python source code for dangerous patterns.
// It returns a ValidationResult. If OK is false, Violations contains the reasons.
func (sv *SandboxValidator) Validate(code string) ValidationResult {
	var violations []string

	// Size check
	if len(code) > sv.maxCodeLen {
		violations = append(violations, fmt.Sprintf("code exceeds max size of %d bytes (got %d)", sv.maxCodeLen, len(code)))
	}

	// Check for blocked imports
	lines := strings.Split(code, "\n")
	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Check import statements
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
			for _, blocked := range sv.blockedImports {
				// Match "import X" or "from X" or "import X.Y"
				if strings.HasPrefix(trimmed, "import "+blocked) ||
					strings.HasPrefix(trimmed, "from "+blocked) ||
					strings.Contains(trimmed, " import "+blocked) {
					violations = append(violations, fmt.Sprintf("line %d: blocked import '%s'", lineNum+1, blocked))
				}
			}
		}
	}

	// Check for blocked patterns (regex-based)
	for i, pattern := range sv.blockedPatterns {
		if pattern.MatchString(code) {
			violations = append(violations, fmt.Sprintf("blocked pattern: %s", sv.blockedPatternDescriptions[i]))
		}
	}

	return ValidationResult{
		OK:         len(violations) == 0,
		Violations: violations,
	}
}
