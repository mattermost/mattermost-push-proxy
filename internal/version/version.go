package version

import (
	"fmt"
	"runtime"
	"strings"
	"text/tabwriter"
)

// Base version information.
//
// This is the fallback data used when version information from git is not
// provided via go ldflags (e.g. via Makefile).
var (
	// Output of "git describe". The prerequisite is that the branch should be
	// tagged using the correct versioning strategy.
	gitVersion string = "devel"
	// SHA1 from git, output of $(git rev-parse HEAD)
	gitCommit = "unknown"
	// State of git tree, either "clean" or "dirty"
	gitTreeState = "unknown"
	// Build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
	buildDate = "unknown"
)

func GetVersion() error {
	v := VersionInfo()
	res := v.String()

	fmt.Println(res)
	return nil
}

type Info struct {
	GitVersion   string
	GitCommit    string
	GitTreeState string
	BuildDate    string
	GoVersion    string
	Compiler     string
	Platform     string
}

func VersionInfo() Info {
	// These variables typically come from -ldflags settings and in
	// their absence fallback to the global defaults set above.
	return Info{
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns the string representation of the version info
func (i *Info) String() string {
	b := strings.Builder{}
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "GitVersion:\t%s\n", i.GitVersion)
	fmt.Fprintf(w, "GitCommit:\t%s\n", i.GitCommit)
	fmt.Fprintf(w, "GitTreeState:\t%s\n", i.GitTreeState)
	fmt.Fprintf(w, "BuildDate:\t%s\n", i.BuildDate)
	fmt.Fprintf(w, "GoVersion:\t%s\n", i.GoVersion)
	fmt.Fprintf(w, "Compiler:\t%s\n", i.Compiler)
	fmt.Fprintf(w, "Platform:\t%s\n", i.Platform)

	w.Flush() // #nosec
	return b.String()
}
