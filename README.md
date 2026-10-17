# TarsLiteLLM

MCP integration for LiteLLM (unified LLM proxy).

## Tools

- `litellm_models` — List available models via LiteLLM
- `litellm_chat` — Chat completion through LiteLLM proxy

## Quick Start

```bash
go build -o tarslitellm
TARS_MCP_ENDPOINT=https://api.example.com TARS_MCP_API_KEY=xxx ./tarslitellm
```

## Environment Variables

- `TARS_MCP_ENDPOINT` — upstream API endpoint
- `TARS_MCP_API_KEY` — upstream API key
- `TARS_MCP_MODE` — `stdio` (default) or `http`

## License

MIT
