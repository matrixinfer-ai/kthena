package tokenization

import "time"

type TokenizeInputType string

const (
	CompletionInput TokenizeInputType = "completion"
	ChatInput       TokenizeInputType = "chat"
)

type TokenizeInput struct {
	Type                TokenizeInputType
	Text                string
	Messages            []ChatMessage
	AddSpecialTokens    bool
	ReturnTokenStrings  bool
	AddGenerationPrompt bool
}

type TokenizeResult struct {
	Count        int      `json:"count"`
	MaxModelLen  int      `json:"max_model_len"`
	Tokens       []int    `json:"tokens"`
	TokenStrings []string `json:"token_strs,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RemoteTokenizerConfig struct {
	Engine             string
	Endpoint           string
	Model              string
	Timeout            time.Duration
	MaxRetries         int
	AddSpecialTokens   bool
	ReturnTokenStrings bool
}

type vllmTokenizeCompletionRequest struct {
	Model            string `json:"model,omitempty"`
	Prompt           string `json:"prompt"`
	AddSpecialTokens *bool  `json:"add_special_tokens,omitempty"`
	ReturnTokenStrs  *bool  `json:"return_token_strs,omitempty"`
}

type vllmTokenizeChatRequest struct {
	Model                string                 `json:"model,omitempty"`
	Messages             []ChatMessage          `json:"messages"`
	AddSpecialTokens     *bool                  `json:"add_special_tokens,omitempty"`
	AddGenerationPrompt  *bool                  `json:"add_generation_prompt,omitempty"`
	ContinueFinalMessage *bool                  `json:"continue_final_message,omitempty"`
	ReturnTokenStrs      *bool                  `json:"return_token_strs,omitempty"`
	ChatTemplate         *string                `json:"chat_template,omitempty"`
	ChatTemplateKwargs   map[string]interface{} `json:"chat_template_kwargs,omitempty"`
	Tools                []interface{}          `json:"tools,omitempty"`
	MMProcessorKwargs    map[string]interface{} `json:"mm_processor_kwargs,omitempty"`
}

type vllmTokenizeResponse struct {
	Count       int      `json:"count"`
	MaxModelLen int      `json:"max_model_len"`
	Tokens      []int    `json:"tokens"`
	TokenStrs   []string `json:"token_strs,omitempty"`
}
