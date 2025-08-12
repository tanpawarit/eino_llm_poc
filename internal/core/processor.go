package core

import (
	"context"
	"fmt"
	"log"
	"time"
	"eino_llm_poc/pkg"
)

// DefaultGraphProcessor implements the GraphProcessor interface
type DefaultGraphProcessor struct {
	nodes  map[string]Node
	config Config
	flow   GraphFlow
}

// NewGraphProcessor creates a new graph processor
func NewGraphProcessor(config Config) GraphProcessor {
	processor := &DefaultGraphProcessor{
		nodes:  make(map[string]Node),
		config: config,
		flow:   config.Graph.DefaultFlow,
	}
	
	return processor
}

// Execute runs the graph flow with the given input
func (g *DefaultGraphProcessor) Execute(ctx context.Context, input ProcessorInput) (*ProcessorOutput, error) {
	startTime := time.Now()
	
	log.Printf("🚀 Starting graph execution for customer: %s", input.CustomerID)
	
	// Prepare initial node input
	nodeInput := NodeInput{
		UserMessage: input.UserMessage,
		CustomerID:  input.CustomerID,
		Metadata:    make(map[string]any),
	}
	
	// Start execution from the start node
	currentNode := g.flow.StartNode
	output := &ProcessorOutput{
		Metadata: make(map[string]any),
	}
	
	// Track execution path
	var executionPath []string
	
	for currentNode != "" && currentNode != "complete" {
		executionPath = append(executionPath, currentNode)
		log.Printf("📍 Executing node: %s", currentNode)
		
		// Get the node
		node, exists := g.nodes[currentNode]
		if !exists {
			return nil, fmt.Errorf("node not found: %s", currentNode)
		}
		
		// Execute the node
		nodeOutput, err := node.Execute(ctx, nodeInput)
		if err != nil {
			log.Printf("❌ Error executing node %s: %v", currentNode, err)
			return nil, fmt.Errorf("error executing node %s: %v", currentNode, err)
		}
		
		// Handle node error (non-fatal)
		if nodeOutput.Error != nil {
			log.Printf("⚠️ Node %s returned error: %v", currentNode, nodeOutput.Error)
			output.Metadata["errors"] = append(getStringSlice(output.Metadata, "errors"), nodeOutput.Error.Error())
		}
		
		// Process node output and update global output
		g.processNodeOutput(currentNode, nodeOutput, output, &nodeInput)
		
		// Check if execution is complete
		if nodeOutput.Complete {
			log.Printf("✅ Graph execution completed at node: %s", currentNode)
			break
		}
		
		// Determine next node
		nextNode := nodeOutput.NextNode
		if nextNode == "" {
			// Use flow edges to determine next node
			nextNode = g.getNextNode(currentNode, nodeOutput)
		}
		
		currentNode = nextNode
	}
	
	// Calculate processing time
	processingTime := time.Since(startTime)
	output.ProcessingTime = processingTime.Milliseconds()
	output.Metadata["execution_path"] = executionPath
	
	log.Printf("🏁 Graph execution completed in %.2fms", float64(processingTime.Nanoseconds())/1000000)
	
	return output, nil
}

// AddNode adds a node to the processor
func (g *DefaultGraphProcessor) AddNode(node Node) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}
	
	nodeName := node.GetName()
	if nodeName == "" {
		return fmt.Errorf("node name cannot be empty")
	}
	
	g.nodes[nodeName] = node
	log.Printf("➕ Added node: %s (type: %s)", nodeName, node.GetType())
	
	return nil
}

// GetNode retrieves a node by name
func (g *DefaultGraphProcessor) GetNode(name string) (Node, error) {
	node, exists := g.nodes[name]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", name)
	}
	return node, nil
}

// SetFlow sets the execution flow
func (g *DefaultGraphProcessor) SetFlow(flow GraphFlow) error {
	if flow.StartNode == "" {
		return fmt.Errorf("start node cannot be empty")
	}
	
	g.flow = flow
	log.Printf("🔀 Updated graph flow, start node: %s", flow.StartNode)
	
	return nil
}

// processNodeOutput processes the output from a node and updates global state
func (g *DefaultGraphProcessor) processNodeOutput(nodeName string, nodeOutput NodeOutput, globalOutput *ProcessorOutput, nodeInput *NodeInput) {
	// Merge node data into global output and next node input
	for key, value := range nodeOutput.Data {
		switch key {
		case "nlu_response":
			globalOutput.NLUAnalysis = value.(*pkg.NLUResponse)
			nodeInput.NLUResult = value.(*pkg.NLUResponse)
		case "response":
			globalOutput.Response = value.(string)
		case "session_updated":
			if updated, ok := value.(bool); ok {
				globalOutput.SessionUpdated = updated
			}
		case "longterm_saved":
			if saved, ok := value.(bool); ok {
				globalOutput.LongtermSaved = saved
			}
		case "tools_executed":
			if tools, ok := value.([]string); ok {
				globalOutput.ToolsExecuted = tools
			}
		case "routing_context":
			nodeInput.RoutingContext = value.(*RoutingContext)
		case "conversation_context":
			if context, ok := value.([]pkg.ConversationMessage); ok {
				nodeInput.ConversationContext = context
			}
		case "tools_required":
			if tools, ok := value.([]string); ok {
				nodeInput.ToolsRequired = tools
			}
		default:
			// Store in metadata
			globalOutput.Metadata[fmt.Sprintf("%s_%s", nodeName, key)] = value
			nodeInput.Metadata[key] = value
		}
	}
	
	log.Printf("📊 Node %s output processed", nodeName)
}

// getNextNode determines the next node based on flow edges and conditions
func (g *DefaultGraphProcessor) getNextNode(currentNode string, nodeOutput NodeOutput) string {
	edges, exists := g.flow.Edges[currentNode]
	if !exists || len(edges) == 0 {
		return "complete"
	}
	
	// Sort edges by priority and evaluate conditions
	for _, edge := range g.sortEdgesByPriority(edges) {
		if g.evaluateCondition(edge.Condition, nodeOutput) {
			return edge.To
		}
	}
	
	// Default to first edge if no conditions match
	if len(edges) > 0 {
		return edges[0].To
	}
	
	return "complete"
}

// sortEdgesByPriority sorts edges by priority (lower number = higher priority)
func (g *DefaultGraphProcessor) sortEdgesByPriority(edges []GraphEdge) []GraphEdge {
	// Simple bubble sort by priority
	sorted := make([]GraphEdge, len(edges))
	copy(sorted, edges)
	
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].Priority > sorted[j+1].Priority {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	return sorted
}

// evaluateCondition evaluates a condition against node output
func (g *DefaultGraphProcessor) evaluateCondition(condition map[string]any, nodeOutput NodeOutput) bool {
	if len(condition) == 0 {
		return true // No condition = always true
	}
	
	// Simple condition evaluation - check if all conditions are met
	for key, expectedValue := range condition {
		actualValue, exists := nodeOutput.Data[key]
		if !exists {
			return false
		}
		
		// Simple equality check
		if actualValue != expectedValue {
			return false
		}
	}
	
	return true
}

// Helper function to safely get string slice from metadata
func getStringSlice(metadata map[string]any, key string) []string {
	if value, exists := metadata[key]; exists {
		if slice, ok := value.([]string); ok {
			return slice
		}
	}
	return []string{}
}

// Note: Using pkg.* types for consistency