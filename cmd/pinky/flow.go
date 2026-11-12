package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DIYgod/pinky/internal/api"
	"github.com/DIYgod/pinky/internal/flow"
	"github.com/DIYgod/pinky/internal/mcp"
)

// runFlow is `pinky flow`: flows (docs/notes/flows.md) from the command
// line, for scripts and CI. Replays go through the running gateway as an
// agent named "Pinky Flow", over the same socket and with the same rules
// and approvals as any agent: the command line is no way around them.
func runFlow(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: pinkyctl flow run|list|show|check|parts …")
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "run":
		return runFlowRun(rest)
	case "list":
		return runFlowList(rest)
	case "show":
		return runFlowShow(rest)
	case "check":
		return runFlowCheck(rest)
	case "parts":
		return runFlowParts(rest)
	default:
		return fmt.Errorf("unknown command %q; use run, list, show, check or parts", cmd)
	}
}

// flowStore opens the folder the gateway keeps flows in.
func flowStore(fake bool) (*flow.Store, error) {
	dirs, err := api.DefaultDirs()
	if err != nil {
		return nil, err
	}
	dir := dirs.Data
	if fake {
		dir = filepath.Join(dir, "fake")
	}
	return flow.NewStore(filepath.Join(dir, "flows")), nil
}

func runFlowList(args []string) error {
	fs := flag.NewFlagSet("pinky flow list", flag.ContinueOnError)
	fake := fs.Bool("fake", false, "the flows of the fake phones' gateway")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := flowStore(*fake)
	if err != nil {
		return err
	}
	flows, broken := st.List()
	if len(flows) == 0 && len(broken) == 0 {
		fmt.Printf("No flows in %s.\n", st.Dir())
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tSTEPS\tAPP\tDESCRIPTION")
	for _, f := range flows {
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n", f.Name, f.Version, len(f.Steps), f.App, f.Description)
	}
	w.Flush()
	for _, name := range broken {
		fmt.Fprintf(os.Stderr, "%s: not a valid flow; see `pinkyctl flow check %s`\n", name, filepath.Join(st.Dir(), name+".yaml"))
	}
	return nil
}

// runFlowParts is `pinky flow parts [app]`: the apps Pinky keeps parts
// of, or one app's parts.
func runFlowParts(args []string) error {
	fs := flag.NewFlagSet("pinky flow parts", flag.ContinueOnError)
	fake := fs.Bool("fake", false, "the parts of the fake phones' gateway")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: pinkyctl flow parts [app]")
	}
	st, err := flowStore(*fake)
	if err != nil {
		return err
	}
	apps := st.PartApps()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	if fs.NArg() == 0 {
		if len(apps) == 0 {
			fmt.Printf("No parts in %s.\n", filepath.Join(st.Dir(), "parts"))
			return nil
		}
		fmt.Fprintln(w, "APP\tPARTS")
		for _, a := range apps {
			if ps, err := st.Parts(a); err == nil {
				fmt.Fprintf(w, "%s\t%d\n", a, len(ps.Parts))
			}
		}
		return nil
	}
	want, app := strings.ToLower(fs.Arg(0)), ""
	for _, a := range apps {
		if l := strings.ToLower(a); l == want || app == "" && strings.Contains(l, want) {
			app = a
		}
	}
	if app == "" {
		return fmt.Errorf("no parts are kept for an app like %q; `pinkyctl flow parts` lists the apps", fs.Arg(0))
	}
	ps, err := st.Parts(app)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "ID\tSEEN\tLAST\tPART\tAFTER")
	for _, p := range ps.Sorted() {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", p.ID, p.Seen, p.Last, p.Text(), strings.Join(p.After, " "))
	}
	return nil
}

func runFlowShow(args []string) error {
	fs := flag.NewFlagSet("pinky flow show", flag.ContinueOnError)
	fake := fs.Bool("fake", false, "the flows of the fake phones' gateway")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: pinkyctl flow show <name>")
	}
	st, err := flowStore(*fake)
	if err != nil {
		return err
	}
	f, err := st.Load(fs.Arg(0))
	if err != nil {
		return err
	}
	data, err := f.Marshal()
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

// runFlowCheck validates flow files without a phone, for editors and CI.
func runFlowCheck(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: pinkyctl flow check <file.yaml>…")
	}
	bad := 0
	for _, path := range args {
		data, err := os.ReadFile(path)
		if err == nil {
			_, err = flow.Parse(data)
		}
		if err != nil {
			bad++
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			continue
		}
		fmt.Printf("%s: ok\n", path)
	}
	if bad > 0 {
		return fmt.Errorf("%d of %d files are not valid flows", bad, len(args))
	}
	return nil
}

// paramFlags collects repeated --param name=value.
type paramFlags map[string]string

func (p paramFlags) String() string { return "" }

func (p paramFlags) Set(v string) error {
	name, value, ok := strings.Cut(v, "=")
	if !ok || name == "" {
		return errors.New("write name=value")
	}
	p[name] = value
	return nil
}

func runFlowRun(args []string) error {
	fs := flag.NewFlagSet("pinky flow run", flag.ContinueOnError)
	params := paramFlags{}
	fs.Var(params, "param", "a flow parameter as name=value; repeat for more")
	device := fs.String("device", "", "the phone, by id or name; optional when one is connected")
	task := fs.String("task", "", "the task name shown in Pinky; default names the flows")
	junit := fs.String("junit", "", "write a JUnit XML report to this file")
	fake := fs.Bool("fake", false, "when no gateway runs, start one with fake phones")
	wait := fs.Duration("approval-wait", 10*time.Minute, "how long to wait in all for the person's approvals")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("usage: pinkyctl flow run [flags] <name or file.yaml>…")
	}
	// Read and check every file before touching a phone.
	type target struct {
		name string         // as given
		args map[string]any // phone_flow_run's flow or yaml
	}
	var targets []target
	for _, arg := range fs.Args() {
		if !strings.HasSuffix(arg, ".yaml") && !strings.HasSuffix(arg, ".yml") && !strings.ContainsAny(arg, `/\`) {
			targets = append(targets, target{arg, map[string]any{"flow": arg}})
			continue
		}
		data, err := os.ReadFile(arg)
		if err != nil {
			return err
		}
		f, err := flow.Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", arg, err)
		}
		targets = append(targets, target{f.Name, map[string]any{"yaml": string(data)}})
	}

	cs, err := flowClient(*fake)
	if err != nil {
		return err
	}
	defer cs.Close()
	deadline := time.Now().Add(*wait)

	title := *task
	if title == "" {
		names := make([]string, len(targets))
		for i, t := range targets {
			names[i] = t.name
		}
		title = "Run " + strings.Join(names, ", ")
	}
	connect := map[string]any{"task": title}
	if *device != "" {
		connect["device"] = *device
	}
	// A new agent's first connection waits for the person's approval.
	for {
		text, isErr, err := callText(cs, "phone_connect", connect)
		if err != nil {
			return err
		}
		status := firstLineOf(text)
		if id := approvalIDRE.FindStringSubmatch(status); !isErr && strings.HasPrefix(status, "approval_pending") && id != nil {
			if time.Now().After(deadline) {
				return errors.New("nobody approved the connection in Pinky")
			}
			fmt.Fprintln(os.Stderr, "Waiting for you to approve “Pinky Flow” in Pinky…")
			connect["approval_id"] = id[1]
			continue
		}
		if isErr || !strings.HasPrefix(status, "settled") {
			return fmt.Errorf("the phone is not ready:\n%s", text)
		}
		break
	}

	var suites []junitSuite
	failed := 0
	for _, t := range targets {
		out, err := replay(cs, t.args, params, deadline)
		if err != nil {
			return err
		}
		printReplay(os.Stdout, t.name, out)
		suites = append(suites, junitOf(t.name, out))
		if out.Status != "settled" {
			failed++
			break // the phone is not where the next flow expects it
		}
	}
	outcome, summary := "done", fmt.Sprintf("%d of %d flows ran", len(suites)-failed, len(targets))
	if failed > 0 {
		outcome = "failed"
	}
	_, _, _ = callText(cs, "phone_release", map[string]any{"outcome": outcome, "summary": summary})

	if *junit != "" {
		data, err := xml.MarshalIndent(junitReport{Suites: suites}, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*junit, append([]byte(xml.Header), append(data, '\n')...), 0o644); err != nil {
			return err
		}
	}
	if failed > 0 {
		return errors.New(summary)
	}
	return nil
}

var approvalIDRE = regexp.MustCompile(`approval_id: (\S+)`)

func firstLineOf(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

// flowClient connects to the gateway as an MCP client, starting the
// gateway when none runs, as `pinky mcp` does.
func flowClient(fake bool) (*sdk.ClientSession, error) {
	dirs, err := api.DefaultDirs()
	if err != nil {
		return nil, err
	}
	conn, err := api.Dial(dirs)
	if err != nil {
		if err := startDaemon(dirs, fake); err != nil {
			return nil, fmt.Errorf("start the gateway: %w", err)
		}
		if conn, err = waitForGateway(dirs, 20*time.Second); err != nil {
			return nil, fmt.Errorf("the gateway did not start (see %s): %w", dirs.DaemonLog(), err)
		}
	}
	tok, err := api.EnsureToken(dirs.TokenFile())
	if err != nil {
		conn.Close()
		return nil, err
	}
	br, err := greetMCP(conn, tok)
	if err != nil {
		conn.Close()
		return nil, err
	}
	cl := sdk.NewClient(&sdk.Implementation{Name: "pinky-flow", Title: "Pinky Flow", Version: version}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cs, err := cl.Connect(ctx, &sdk.IOTransport{Reader: io.NopCloser(br), Writer: conn}, nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return cs, nil
}

func callTool(cs *sdk.ClientSession, tool string, args map[string]any) (*sdk.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tool, err)
	}
	return res, nil
}

func callText(cs *sdk.ClientSession, tool string, args map[string]any) (string, bool, error) {
	res, err := callTool(cs, tool, args)
	if err != nil {
		return "", false, err
	}
	return resultText(res), res.IsError, nil
}

func resultText(res *sdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// replay runs one flow to its end. A step that waits for the person is
// repeated with its approval id until they answer; the steps of every
// call add up to one report.
func replay(cs *sdk.ClientSession, target map[string]any, params map[string]string, deadline time.Time) (mcp.FlowOutput, error) {
	args := map[string]any{}
	for k, v := range target {
		args[k] = v
	}
	if len(params) > 0 {
		args["params"] = map[string]string(params)
	}
	var total mcp.FlowOutput
	for {
		res, err := callTool(cs, "phone_flow_run", args)
		if err != nil {
			return total, err
		}
		var out mcp.FlowOutput
		raw, err := json.Marshal(res.StructuredContent)
		if err == nil {
			err = json.Unmarshal(raw, &out)
		}
		if err != nil || out.Status == "" {
			return total, fmt.Errorf("phone_flow_run returned no report:\n%s", resultText(res))
		}
		steps := total.Steps
		elapsed := total.ElapsedMS + out.ElapsedMS
		total = out
		total.ElapsedMS = elapsed
		pending := out.Status == "approval_pending" && out.ApprovalID != "" && len(out.Steps) > 0
		if pending {
			// The step that waits runs again in the next call.
			total.Steps = append(steps, out.Steps[:len(out.Steps)-1]...)
		} else {
			total.Steps = append(steps, out.Steps...)
		}
		if !pending {
			return total, nil
		}
		if time.Now().After(deadline) {
			total.Steps = append(total.Steps, out.Steps[len(out.Steps)-1])
			total.Message = "Nobody answered the approval in Pinky. " + out.Message
			return total, nil
		}
		fmt.Fprintf(os.Stderr, "Step %d waits for your approval in Pinky…\n", out.Steps[len(out.Steps)-1].Index)
		args["from_step"] = out.Steps[len(out.Steps)-1].Index
		args["approval_id"] = out.ApprovalID
	}
}

func printReplay(w io.Writer, name string, out mcp.FlowOutput) {
	for _, st := range out.Steps {
		mark := "ok  "
		switch st.Status {
		case "ok":
		case "skipped":
			mark = "skip"
		default:
			mark = "FAIL"
		}
		fmt.Fprintf(w, "%s  %s  step %d: %s (%d ms)\n", mark, name, st.Index, st.Text, st.ElapsedMS)
	}
	if out.Status == "settled" {
		fmt.Fprintf(w, "%s: %d steps in %d ms\n", name, len(out.Steps), out.ElapsedMS)
		return
	}
	fmt.Fprintf(w, "%s: %s\n%s\n", name, out.Status, out.Message)
}

// JUnit XML, as CI systems read it: one suite per flow, one case per step.
type junitReport struct {
	XMLName xml.Name     `xml:"testsuites"`
	Suites  []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	Skipped   *junitSkipped `xml:"skipped,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

type junitSkipped struct {
	Message string `xml:"message,attr,omitempty"`
}

func junitOf(name string, out mcp.FlowOutput) junitSuite {
	seconds := func(ms int64) string { return fmt.Sprintf("%.3f", float64(ms)/1000) }
	s := junitSuite{Name: name, Time: seconds(out.ElapsedMS)}
	for _, st := range out.Steps {
		c := junitCase{Name: fmt.Sprintf("step %d: %s", st.Index, st.Text), Classname: name, Time: seconds(st.ElapsedMS)}
		switch st.Status {
		case "ok":
		case "skipped":
			c.Skipped = &junitSkipped{Message: st.Detail}
			s.Skipped++
		default:
			c.Failure = &junitFailure{Message: st.Status, Text: out.Message}
			s.Failures++
		}
		s.Cases = append(s.Cases, c)
	}
	// The steps after the one that stopped the replay did not run.
	if n := len(out.Steps); n > 0 && out.Status != "settled" {
		for i := out.Steps[n-1].Index + 1; i <= out.Total; i++ {
			s.Cases = append(s.Cases, junitCase{Name: fmt.Sprintf("step %d", i), Classname: name, Time: "0.000",
				Skipped: &junitSkipped{Message: "not run"}})
			s.Skipped++
		}
	}
	if len(out.Steps) == 0 && out.Status != "settled" {
		// The replay never started: a flow that is not there, bad parameters.
		s.Cases = append(s.Cases, junitCase{Name: "start", Classname: name, Time: "0.000",
			Failure: &junitFailure{Message: out.Status, Text: out.Message}})
		s.Failures++
	}
	s.Tests = len(s.Cases)
	return s
}
