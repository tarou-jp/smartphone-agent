package mcp

import "github.com/DIYgod/pinky/internal/gateway"

// FlowOutput is phone_flow_run's structured result, for scripts and CI:
// the same facts as the text, without parsing it.
type FlowOutput struct {
	Status     string           `json:"status"` // settled, mismatch, approval_pending, …, or an error code
	Error      bool             `json:"error,omitempty"`
	Message    string           `json:"message,omitempty"`
	ApprovalID string           `json:"approval_id,omitempty"`
	Flow       string           `json:"flow,omitempty"`
	Version    int              `json:"version,omitempty"`
	Total      int              `json:"total,omitempty"` // steps the flow has
	ElapsedMS  int64            `json:"elapsed_ms"`
	Steps      []FlowOutputStep `json:"steps,omitempty"` // the steps that ran
}

// FlowOutputStep is one step of a replay.
type FlowOutputStep struct {
	Index     int    `json:"index"`  // counted from 1
	Text      string `json:"text"`   // what the step does
	Status    string `json:"status"` // ok, skipped, mismatch, or what stopped the replay
	Detail    string `json:"detail,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// FlowOutputOf is a replay's result in its structured form.
func FlowOutputOf(res *gateway.Result) FlowOutput {
	out := FlowOutput{Status: string(res.Status), Message: res.Message, ApprovalID: res.ApprovalID}
	if res.IsError() {
		out.Status, out.Error = string(res.Error), true
	}
	if f := res.Flow; f != nil {
		out.Flow, out.Version, out.Total, out.ElapsedMS = f.Name, f.Version, f.Total, f.Elapsed.Milliseconds()
		for _, st := range f.Steps {
			out.Steps = append(out.Steps, FlowOutputStep{Index: st.Index, Text: st.Text, Status: st.Status,
				Detail: st.Detail, ElapsedMS: st.Elapsed.Milliseconds()})
		}
	}
	return out
}
