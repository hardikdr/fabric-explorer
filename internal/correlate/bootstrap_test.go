package correlate

import "testing"

func TestShellBootstrapLiteralIdentity(t *testing.T) {
	for _, tc := range []struct {
		script string
		count  int
	}{
		{"nsenter --target 1 --net /fabriclet --name leaf-node-1 --backend configdb", 1},
		{"exec /usr/local/bin/fabriclet-sonic --name=spine-node-2 --backend configdb", 1},
		{"/fabriclet --name $NODE", 0}, {"/fabriclet --name $(cat credential)", 0},
		{"unrelated --name leaf-node-1", 0}, {"/fabriclet --name leaf-node-1; /fabriclet --name leaf-node-2", 2},
	} {
		if got := shellBootstrapNames(tc.script); len(got) != tc.count {
			t.Fatalf("%q: %v", tc.script, got)
		}
	}
}
