package nodes

import (
	"context"
	"fmt"
	"log"
	"eino_llm_poc/pkg"
	"eino_llm_poc/internal/core"
)

// ResponseNode handles LLM response generation
type ResponseNode struct {
	config core.Config
}

// NewResponseNode creates a new response generation node
func NewResponseNode(config core.Config) *ResponseNode {
	return &ResponseNode{
		config: config,
	}
}

// Execute generates a response based on NLU analysis and context
func (r *ResponseNode) Execute(ctx context.Context, input core.NodeInput) (core.NodeOutput, error) {
	log.Printf("💬 Generating response for customer: %s", input.CustomerID)

	// Get NLU result
	if input.NLUResult == nil {
		return core.NodeOutput{Error: fmt.Errorf("no NLU result available")}, nil
	}

	// Generate response based on primary intent
	response := r.generateResponse(input)

	// Check if tools are needed
	needTools := r.checkToolsRequired(input)

	// Prepare output
	output := core.NodeOutput{
		Data: map[string]any{
			"response":       response,
			"need_tools":     needTools,
			"response_type":  "direct", // or "tool_assisted"
		},
		Complete: !needTools, // Complete if no tools needed
	}

	if needTools {
		output.NextNode = "tools"
		output.Data["tools_required"] = r.getRequiredTools(input)
	} else {
		output.NextNode = "complete"
	}

	log.Printf("✅ Response generated: length=%d, tools_needed=%v", len(response), needTools)

	return output, nil
}

// GetName returns the node name
func (r *ResponseNode) GetName() string {
	return "response"
}

// GetType returns the node type
func (r *ResponseNode) GetType() core.NodeType {
	return core.NodeTypeResponse
}

// generateResponse creates a response based on NLU analysis
func (r *ResponseNode) generateResponse(input core.NodeInput) string {
	nlu := input.NLUResult
	
	// Simple response generation based on primary intent
	switch nlu.PrimaryIntent {
	case "greet":
		return "สวัสดีครับ! มีอะไรให้ช่วยเหลือไหมครับ?"
		
	case "purchase_intent":
		productEntities := r.extractEntitiesByType(nlu.Entities, "product")
		if len(productEntities) > 0 {
			return fmt.Sprintf("เข้าใจแล้วครับ คุณสนใจซื้อ%s ใช่ไหมครับ? ต้องการทราบรายละเอียดเพิ่มเติมไหมครับ?", productEntities[0].Value)
		}
		return "เข้าใจแล้วครับ คุณสนใจซื้อสินค้า ต้องการทราบรายละเอียดเพิ่มเติมไหมครับ?"
		
	case "inquiry_intent":
		return "ครับ ผมพร้อมตอบคำถามครับ มีอะไรอยากทราบเพิ่มเติมไหมครับ?"
		
	case "ask_price":
		return "สำหรับราคาสินค้า ผมจะช่วยเช็คให้นะครับ รอสักครู่เดียวครับ"
		
	case "support_intent":
		return "ครับ ทีมซัพพอร์ตพร้อมช่วยเหลือครับ มีปัญหาอะไรให้ช่วยแก้ไขไหมครับ?"
		
	case "complain_intent", "complaint":
		return "ขออภัยมากครับ ที่มีปัญหาเกิดขึ้น ผมจะช่วยแก้ไขให้เร็วที่สุดครับ สามารถเล่ารายละเอียดเพิ่มเติมได้ไหมครับ?"
		
	case "cancel_order":
		return "เข้าใจครับ เรื่องการยกเลิกคำสั่งซื้อ ให้ผมช่วยเช็คและดำเนินการให้นะครับ"
		
	case "compare_product":
		return "ครับ ผมจะช่วยเปรียบเทียบสินค้าให้ครับ มีผลิตภัณฑ์ไหนเป็นพิเศษที่อยากทราบไหมครับ?"
		
	default:
		return "ขอบคุณสำหรับข้อความครับ ผมจะช่วยเหลือให้ดีที่สุดครับ"
	}
}

// checkToolsRequired determines if tools are needed for this request
func (r *ResponseNode) checkToolsRequired(input core.NodeInput) bool {
	nlu := input.NLUResult
	
	// Tools required for specific intents
	toolRequiredIntents := map[string]bool{
		"ask_price":       true, // Need product database lookup
		"purchase_intent": true, // Need product database lookup
		"inquiry_intent":  true, // Need product database lookup
		"compare_product": true, // Need product comparison tools
		"cancel_order":    true, // Need order management tools
		"support_intent":  true, // Might need ticket system
	}
	
	return toolRequiredIntents[nlu.PrimaryIntent]
}

// getRequiredTools returns the list of tools needed
func (r *ResponseNode) getRequiredTools(input core.NodeInput) []string {
	nlu := input.NLUResult
	
	switch nlu.PrimaryIntent {
	case "ask_price":
		return []string{"product_database", "price_lookup"}
	case "purchase_intent":
		return []string{"product_database"}
	case "inquiry_intent":
		return []string{"product_database"}
	case "compare_product":
		return []string{"product_database", "comparison_engine"}
	case "cancel_order":
		return []string{"order_management", "cancellation_service"}
	case "support_intent":
		return []string{"ticket_system", "support_database"}
	default:
		return []string{}
	}
}

// extractEntitiesByType extracts entities of a specific type
func (r *ResponseNode) extractEntitiesByType(entities []pkg.Entity, entityType string) []pkg.Entity {
	var result []pkg.Entity
	for _, entity := range entities {
		if entity.Type == entityType {
			result = append(result, entity)
		}
	}
	return result
}