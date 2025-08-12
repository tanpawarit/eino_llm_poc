# Eino Graph Implementation Examples

This directory contains examples showing how to implement graph-like workflows using the Eino framework from CloudWego.

## What is Eino?

Eino is CloudWego's framework for building LLM applications with features like:
- **Chain Composition**: Template → ChatModel workflows
- **Type Safety**: Compile-time type checking
- **Stream Support**: Built-in streaming capabilities
- **Component System**: Extensible and composable components

## Examples Overview

### 1. `eino_graph_example.go` - Basic Chain Workflow
Shows how to create a simple workflow using Eino chains:
- Intent analysis chain
- Response generation chain
- Sequential processing with state management

**Key Features:**
- Template-based prompt engineering using `prompt.FromMessages`
- Chain composition with `compose.NewChain()`
- JSON response parsing with fallbacks
- Multi-step workflow execution

### 2. `eino_simple_workflow.go` - Structured Workflow
Demonstrates a more structured approach:
- Step-by-step execution tracking
- Performance monitoring
- Error handling at each step
- Detailed workflow results

**Key Features:**
- Workflow state management
- Step timing and performance metrics
- Comprehensive error handling
- Structured result reporting

### 3. `eino_advanced_graph.go` - Complex Workflows (Work in Progress)
More advanced patterns including:
- Tool integration
- Complex branching logic
- Advanced state management
- Retry mechanisms

## Core Eino Patterns Used

### 1. Chain Composition
```go
chain, err := compose.NewChain[map[string]any, *schema.Message]().
    AppendChatTemplate(template).
    AppendChatModel(model).
    Compile(ctx)
```

### 2. Template Creation
```go
messages := []schema.MessagesTemplate{
    schema.SystemMessage(systemText),
    schema.UserMessage(userText),
}
template := prompt.FromMessages(schema.FString, messages...)
```

### 3. Chain Execution
```go
result, err := chain.Invoke(ctx, map[string]any{
    "key": "value",
})
```

## Setup and Running

### Prerequisites
1. Go 1.24.5 or later
2. Valid OpenRouter API key (or other OpenAI-compatible API)
3. Internet connection for API calls

### Environment Setup
1. Copy `.env.example` to `.env` if it exists, or create a new `.env` file:
```bash
OPENROUTER_API_KEY=your_actual_api_key_here
```

2. Install dependencies:
```bash
go mod tidy
```

### Running Examples

#### Basic Chain Workflow
```bash
go run examples/eino_graph_example.go
```

#### Simple Workflow
```bash
go run examples/eino_simple_workflow.go
```

#### Run All Examples
```bash
chmod +x run_eino_demos.sh
./run_eino_demos.sh
```

## Expected Output

### Basic Chain Workflow
```
⚙️ Creating Eino processing chains...
✅ Eino chains created successfully!

🚀 Eino Chain Workflow Demo
============================

🧪 Test 1
---
🔄 Processing: สวัสดีครับ
🧠 Step 1: Analyzing intent...
   Detected intent: greeting (0.95 confidence)
📝 Step 2: Generating response...
   Generated response: สวัสดีครับ! ยินดีที่ได้รู้จักนะครับ มีอะไรให้ช่วยเหลือไหมครับ?
✅ Processing completed successfully!
📊 Results:
   Intent: greeting
   Steps: 2
   Final Response: สวัสดีครับ! ยินดีที่ได้รู้จักนะครับ มีอะไรให้ช่วยเหลือไหมครับ?
```

### Simple Workflow
```
🔄 Eino Simple Workflow Demo
============================

🧪 Test 1: Greeting
Input: สวัสดีครับ
---
🚀 Starting workflow for: สวัสดีครับ
📊 Step 1 Complete: Intent = greeting (245.67ms)
📝 Step 2 Complete: Response generated (892.34ms)
✅ Workflow completed successfully (1138.01ms total)
🎯 Final Result:
   Intent: greeting
   Response: สวัสดีครับ! ยินดีที่ได้รู้จักนะครับ
   Steps: 2
   Total Time: 1138.01ms
   Success: true
📈 Step Breakdown:
   1. intent_detection ✅ (245.67ms)
   2. content_processing ✅ (892.34ms)
```

## Key Differences from Traditional Graph Frameworks

Unlike traditional graph frameworks (like LangGraph), Eino focuses on:

1. **Chain Composition**: Sequential processing through chains rather than node-based graphs
2. **Type Safety**: Compile-time type checking with generics
3. **Template-First**: Prompt templates are first-class citizens
4. **Component System**: Extensible components that can be composed horizontally and vertically

## Integration with Your Existing Code

The existing codebase (`internal/core/processor.go`) implements a custom graph processor. The Eino examples show how to:

1. **Replace custom nodes** with Eino chains
2. **Simplify state management** using Eino's built-in patterns
3. **Improve type safety** with Eino's generic types
4. **Enhance performance** with Eino's optimized execution

## Troubleshooting

### Common Issues

1. **Invalid API Key**: Make sure your `.env` file contains a valid API key
2. **Network Issues**: Ensure internet connectivity for API calls
3. **Go Version**: Requires Go 1.24.5 or later
4. **Dependencies**: Run `go mod tidy` if you encounter import issues

### Debug Mode

Add this to your code for detailed logging:
```go
import "log"

log.SetLevel(log.DebugLevel)
```

## Next Steps

1. **Try the examples** with your own API key
2. **Modify the templates** to see how different prompts affect results
3. **Add your own chains** for specific use cases
4. **Integrate patterns** into your existing codebase
5. **Explore tool integration** with the advanced examples

## Resources

- [Eino Documentation](https://www.cloudwego.io/docs/eino/overview/eino_open_source/)
- [CloudWego Eino GitHub](https://github.com/cloudwego/eino)
- [OpenAI API Documentation](https://platform.openai.com/docs/api-reference)
- [OpenRouter API](https://openrouter.ai/) (OpenAI-compatible API gateway)

## Contributing

Feel free to add more examples or improve existing ones! The goal is to demonstrate various Eino patterns and best practices for building LLM applications.