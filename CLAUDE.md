# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Code Style Guidelines

### Writing Simple and Readable Code
- **Simplicity First**: Always choose the simplest solution that works correctly
- **Readable Code**: Write code that tells a clear story - avoid clever tricks that obscure meaning
- **Clear Naming**: Use descriptive variable and function names that explain intent
- **Single Responsibility**: Each function should do one thing well
- **Minimal Comments**: Code should be self-explanatory; add comments only when necessary to explain "why", not "what"
- **Consistent Style**: Follow Go conventions and maintain consistency throughout the codebase
- **Small Functions**: Keep functions short and focused - aim for functions that fit on a single screen
- **Avoid Deep Nesting**: Use early returns and guard clauses to reduce nesting levels

## Development Commands

### Build and Run
- `go run main.go` - Run the main NLU processing application
- `go run test/main.go` - Run the simplified test version
- `go build` - Build the main application binary
- `go test ./...` - Run all tests

### Testing with OpenRouter API
All examples and tests use OpenRouter API. Set the API key as environment variable:
```bash
OPENROUTER_API_KEY=dummy go run main.go
OPENROUTER_API_KEY=dummy go run test/main.go
OPENROUTER_API_KEY=dummy go run examples/<path>/main.go
```

### Environment Setup
1. Copy `.env.example` to `.env` and configure:
   - `OPENROUTER_API_KEY` - Your OpenRouter API key
   - `REDIS_URL` - Redis connection string (optional for some examples)
2. Configure `config.yaml` with NLU settings (model, temperature, etc.)

## Architecture Overview

### Core System Design
This is an **NLU (Natural Language Understanding) processing system** built on CloudWego Eino framework using a **graph-based composition pattern**. The system processes user messages to extract intents, entities, languages, and sentiment.

### Main Components

**Graph Processing Pipeline:**
- **InputTransformer** → **ChatModel** → **OutputParser**
- Uses Eino's `compose.Graph` with typed nodes and edges
- Supports both synchronous (`Invoke`) and streaming execution patterns

**NLU Processing Architecture:**
- `src/llm/nlu/` - Core NLU processing with structured output parsing
- `src/model/` - Data models for NLU requests/responses and configuration
- Structured tuple-based output format with configurable delimiters (`##`, `<||>`, `<|COMPLETE|>`)

**Configuration System:**
- YAML-based configuration (`config.yaml`) loaded via `src/config.go`
- Runtime template injection for intents/entities lists
- Environment variable support via godotenv

**Model Integration:**
- CloudWego Eino framework with OpenAI-compatible models via OpenRouter
- Configurable model parameters (temperature, max_tokens, etc.)
- Supports various OpenRouter model providers

### Key Patterns

**Graph Composition Pattern:**
```go
g := compose.NewGraph[InputType, OutputType]()
g.AddLambdaNode(nodeName, transformFunction)
g.AddChatModelNode(nodeName, model)
g.AddEdge(compose.START, firstNode)
```

**NLU Output Structure:**
- Structured tuples: `(type<||>name<||>value<||>confidence<||>metadata)`
- Types: intent, entity, language, sentiment
- Confidence scores and priority-based ranking system

**Template Processing:**
- System prompts with runtime configuration injection
- Conversation context formatting for analysis
- Multi-language support (Thai/English examples)

## Examples Directory Structure

### Agent Examples (`examples/agent/`)
Complex multi-agent systems with different orchestration patterns:

- **deer-go/** - Full-featured research team agent system with coordinator, planners, researchers, and reporters
- **manus/** - Manuscript processing agent example  
- **multiagent/** - Various multi-agent patterns (host/journal, plan-execute, react)

### Compose Examples (`examples/compose/`)
Fundamental Eino composition patterns:

- **chain/** - Simple sequential processing chains
- **graph/** - Graph-based processing with branching, state management, and tool calling
- **workflow/** - Step-by-step workflow examples from simple to advanced field mapping

### Running Examples
Each example has its own main.go and can be run with:
```bash
OPENROUTER_API_KEY=dummy go run examples/<category>/<example>/main.go
```

## Development Notes

### NLU System Specifics
- Supports configurable intent/entity lists via YAML configuration
- Importance scoring system with threshold-based filtering
- Multi-language detection with primary language identification
- Structured metadata extraction with validation limits

### Error Handling
- Comprehensive input validation with UTF-8 checking
- Graceful degradation for parsing failures
- Metadata size and field count limits for safety
- Warning-based logging for non-critical parsing issues

### Testing Approach
- The `test/` directory contains simplified versions for rapid testing
- Main application includes comprehensive NLU response parsing
- Examples demonstrate various usage patterns and complexity levels

### Configuration Management
- YAML-based NLU configuration with runtime template injection
- Environment variable support for API keys and external services
- Separate configuration files for complex examples (deer-go system)