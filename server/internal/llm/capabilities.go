// Package llm holds LLM model capability taxonomy and inference helpers.
//
// The capability tags live in llm_model.capabilities (TEXT[]) and are the
// machine-readable source of truth for a model's ability boundaries — used to
// match models to agent roles. Tags are populated by FetchProviderModels from
// three sources: provider-exposed metadata (OpenRouter), naming heuristics
// (covers providers like DeepSeek/qnaigc that expose nothing), and manual
// curation in the UI. Keep the tag set in sync with the frontend
// LLM_CAPABILITIES tuple in packages/core/types/llm.ts.
package llm

import (
	"regexp"
	"strings"
)

// Capability tags.
const (
	CapReasoning   = "reasoning"
	CapToolUse     = "tool_use"
	CapVision      = "vision"
	CapImageGen    = "image_gen"
	CapCode        = "code"
	CapAudio       = "audio"
	CapEmbedding   = "embedding"
	CapLongContext = "long_context"
)

// LongContextThreshold is the context window (in tokens) at or above which a
// model is tagged long_context.
const LongContextThreshold = 128_000

// RemoteModelMeta is the structured metadata some aggregators (OpenRouter)
// attach to /v1/models entries. All fields optional; nil where the provider
// exposes nothing beyond the bare OpenAI shape.
type RemoteModelMeta struct {
	InputModalities   []string
	OutputModalities  []string
	SupportedParams   []string
	ContextLength     int64
}

// namingRules map a capability to a regexp matched against the lowercased
// model_code. Conservative by design — only high-confidence signals so we
// don't over-claim a capability a model lacks (which would mis-route agents).
// tool_use is intentionally NOT inferred from naming (too error-prone); it is
// only set when the provider declares it via SupportedParams.
var namingRules = []struct {
	cap string
	re  *regexp.Regexp
}{
	{CapReasoning, regexp.MustCompile(`(?:^|[-/_])(o1|o3|o4|reasoner|r1|qwq|thinking)(?:[-/_]|$)`)},
	{CapVision, regexp.MustCompile(`vision|gpt-4o|^claude-3|claude-sonnet|claude-opus|claude-haiku|gemini|qwen[0-9.]*-?vl|glm-4v|pixtral|llava|internvl|minicpm-v|-vl`)},
	{CapImageGen, regexp.MustCompile(`dall-?e|imagen|flux|sd3|sdxl|stable-diffusion|midjourney`)},
	{CapCode, regexp.MustCompile(`coder|codestral|codellama|starcoder|deepseek-coder|qwen.*coder`)},
	{CapAudio, regexp.MustCompile(`whisper|tts-?1|tts-?hd|^audio|speech`)},
	{CapEmbedding, regexp.MustCompile(`embed|text-embedding|^e5|bge-`)},
}

// InferCapabilities derives a model's capability tags from its model_code,
// context window, and any structured metadata the provider exposed. The result
// is the union of all signals; callers merge it with user-curated tags before
// persisting.
func InferCapabilities(modelCode string, contextWindow int32, meta *RemoteModelMeta) []string {
	code := strings.ToLower(modelCode)
	seen := make(map[string]bool)
	add := func(c string) {
		if !seen[c] {
			seen[c] = true
		}
	}

	for _, r := range namingRules {
		if r.re.MatchString(code) {
			add(r.cap)
		}
	}

	// Long context: take the larger of the stored window and provider-reported.
	cw := int64(contextWindow)
	if meta != nil && meta.ContextLength > cw {
		cw = meta.ContextLength
	}
	if cw >= LongContextThreshold {
		add(CapLongContext)
	}

	if meta != nil {
		for _, p := range meta.SupportedParams {
			switch strings.ToLower(p) {
			case "tools", "tool_choice":
				add(CapToolUse)
			case "reasoning":
				add(CapReasoning)
			}
		}
		for _, m := range meta.InputModalities {
			switch strings.ToLower(m) {
			case "image", "images":
				add(CapVision)
			case "audio":
				add(CapAudio)
			}
		}
		for _, m := range meta.OutputModalities {
			switch strings.ToLower(m) {
			case "image", "images":
				add(CapImageGen)
			case "audio":
				add(CapAudio)
			}
		}
	}

	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	return out
}

// InferType maps a capability set to the coarse llm_model.type classifier:
// image_gen→3, audio→5, embedding→4, vision→2, otherwise 1 (LLM对话). Used on
// insert only; re-fetch preserves any user-set type.
func InferType(caps []string) int16 {
	has := func(c string) bool {
		for _, x := range caps {
			if x == c {
				return true
			}
		}
		return false
	}
	switch {
	case has(CapImageGen):
		return 3
	case has(CapAudio):
		return 5
	case has(CapEmbedding):
		return 4
	case has(CapVision):
		return 2
	}
	return 1
}

// MergeCapabilities returns the de-duplicated union of the given capability
// slices, preserving order (a first, then new tags from b). nil-safe.
func MergeCapabilities(a, b []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
