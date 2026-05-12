-- Allow the model marketplace to use the same channel set as New API.

ALTER TABLE IF EXISTS model_marketplace_request_templates
  ALTER COLUMN provider TYPE VARCHAR(50);

ALTER TABLE IF EXISTS model_marketplace_monitors
  ALTER COLUMN provider TYPE VARCHAR(50);

ALTER TABLE IF EXISTS model_marketplace_request_templates
  DROP CONSTRAINT IF EXISTS model_marketplace_request_templates_provider_check;

ALTER TABLE IF EXISTS model_marketplace_monitors
  DROP CONSTRAINT IF EXISTS model_marketplace_monitors_provider_check;

ALTER TABLE IF EXISTS model_marketplace_request_templates
  ADD CONSTRAINT model_marketplace_request_templates_provider_check CHECK (
    provider IN ('openai', 'openai_max', 'ohmygpt', 'custom', 'ails', 'ai_proxy', 'api2gpt', 'aigc2d', 'anthropic', 'aws', 'gemini', 'deepseek', 'azure', 'vertex_ai', 'xai', 'mistral', 'cohere', 'openrouter', 'ollama', 'siliconflow', 'perplexity', 'moonshot', 'ali', 'zhipu_v4', 'baidu', 'baidu_v2', 'tencent', 'xunfei', 'volcengine', 'lingyiwanwu', 'minimax', 'coze', 'ai360', 'xinference', 'dify', 'jina', 'cloudflare', 'palm', 'codex', 'fastgpt', 'ai_proxy_library', 'mokaai', 'midjourney', 'midjourney_plus', 'sunoapi', 'kling', 'jimeng', 'vidu', 'submodel', 'doubao_video', 'sora', 'replicate')
  );

ALTER TABLE IF EXISTS model_marketplace_monitors
  ADD CONSTRAINT model_marketplace_monitors_provider_check CHECK (
    provider IN ('openai', 'openai_max', 'ohmygpt', 'custom', 'ails', 'ai_proxy', 'api2gpt', 'aigc2d', 'anthropic', 'aws', 'gemini', 'deepseek', 'azure', 'vertex_ai', 'xai', 'mistral', 'cohere', 'openrouter', 'ollama', 'siliconflow', 'perplexity', 'moonshot', 'ali', 'zhipu_v4', 'baidu', 'baidu_v2', 'tencent', 'xunfei', 'volcengine', 'lingyiwanwu', 'minimax', 'coze', 'ai360', 'xinference', 'dify', 'jina', 'cloudflare', 'palm', 'codex', 'fastgpt', 'ai_proxy_library', 'mokaai', 'midjourney', 'midjourney_plus', 'sunoapi', 'kling', 'jimeng', 'vidu', 'submodel', 'doubao_video', 'sora', 'replicate')
  );
