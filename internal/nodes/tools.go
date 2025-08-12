package nodes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"eino_llm_poc/internal/core"
	"eino_llm_poc/internal/services"
)

// ToolsNode handles tool execution
type ToolsNode struct {
	productService *services.ProductService
	config         core.Config
}

// NewToolsNode creates a new tools node
func NewToolsNode(config core.Config) *ToolsNode {
	return &ToolsNode{
		productService: services.NewProductService(),
		config:         config,
	}
}

// Execute processes tool execution requests
func (t *ToolsNode) Execute(ctx context.Context, input core.NodeInput) (core.NodeOutput, error) {
	log.Printf("🔧 Executing tools for customer: %s", input.CustomerID)

	var toolResults []string
	var executedTools []string

	// Check which tools are required
	if input.ToolsRequired != nil {
		for _, tool := range input.ToolsRequired {
			result, err := t.executeTool(ctx, tool, input)
			if err != nil {
				log.Printf("❌ Tool %s failed: %v", tool, err)
				continue
			}
			
			toolResults = append(toolResults, result)
			executedTools = append(executedTools, tool)
			log.Printf("✅ Tool %s executed successfully", tool)
		}
	}

	// Combine tool results
	combinedResults := strings.Join(toolResults, "\n\n")

	// Generate final response with tool data
	finalResponse := t.generateResponseWithTools(input, combinedResults, executedTools)

	output := core.NodeOutput{
		Data: map[string]any{
			"response":         finalResponse,
			"tools_executed":   executedTools,
			"tool_results":     combinedResults,
			"response_type":    "tool_assisted",
		},
		Complete: true,
	}

	log.Printf("✅ Tools execution completed: tools=%v", executedTools)
	
	return output, nil
}

// GetName returns the node name
func (t *ToolsNode) GetName() string {
	return "tools"
}

// GetType returns the node type
func (t *ToolsNode) GetType() core.NodeType {
	return core.NodeTypeTools
}

// executeTool executes a specific tool
func (t *ToolsNode) executeTool(ctx context.Context, toolName string, input core.NodeInput) (string, error) {
	switch toolName {
	case "product_database", "price_lookup":
		return t.executeProductTool(ctx, input)
	case "comparison_engine":
		return t.executeComparisonTool(ctx, input)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// executeProductTool searches for products
func (t *ToolsNode) executeProductTool(ctx context.Context, input core.NodeInput) (string, error) {
	// Extract product entities from NLU result
	var searchQuery string
	
	if input.NLUResult != nil {
		// Look for product entities
		for _, entity := range input.NLUResult.Entities {
			if entity.Type == "product" {
				searchQuery = entity.Value
				break
			}
		}
	}
	
	// If no product entity, use keywords from user message
	if searchQuery == "" {
		userMsg := strings.ToLower(input.UserMessage)
		keywords := []string{"โน้ตบุ๊ค", "notebook", "คอมพิวเตอร์", "computer", "mac", "apple", "lenovo", "เมาส์", "mouse"}
		
		for _, keyword := range keywords {
			if strings.Contains(userMsg, keyword) {
				searchQuery = keyword
				break
			}
		}
	}

	// Search products
	products := t.productService.SearchProducts(ctx, searchQuery)
	
	if len(products) == 0 {
		return "ขออภัยครับ ไม่พบสินค้าที่ตรงกับที่ท่านต้องการ", nil
	}

	// Format product results
	result := "🛍️ สินค้าที่พบ:\n\n"
	for i, product := range products {
		if i >= 3 { // Limit to 3 products
			break
		}
		
		stockStatus := "✅ มีสินค้า"
		if !product.InStock {
			stockStatus = "❌ สินค้าหมด"
		}
		
		result += fmt.Sprintf("%d. **%s** (%s)\n", i+1, product.Name, product.Brand)
		result += fmt.Sprintf("   💰 ราคา: %.0f บาท\n", product.Price)
		result += fmt.Sprintf("   📝 %s\n", product.Description)
		result += fmt.Sprintf("   📦 สถานะ: %s\n\n", stockStatus)
	}

	return result, nil
}

// executeComparisonTool compares products
func (t *ToolsNode) executeComparisonTool(ctx context.Context, input core.NodeInput) (string, error) {
	// Get all available products for comparison
	products := t.productService.SearchProducts(ctx, "")
	
	if len(products) < 2 {
		return "ขออภัยครับ ต้องมีสินค้าอย่างน้อย 2 รายการเพื่อเปรียบเทียบ", nil
	}

	// Simple comparison of first 2 products
	p1, p2 := products[0], products[1]
	
	result := "🔍 เปรียบเทียบสินค้า:\n\n"
	result += fmt.Sprintf("**%s** vs **%s**\n\n", p1.Name, p2.Name)
	
	result += "💰 **ราคา:**\n"
	result += fmt.Sprintf("- %s: %.0f บาท\n", p1.Name, p1.Price)
	result += fmt.Sprintf("- %s: %.0f บาท\n\n", p2.Name, p2.Price)
	
	result += "🏢 **ยี่ห้อ:**\n"
	result += fmt.Sprintf("- %s: %s\n", p1.Name, p1.Brand)
	result += fmt.Sprintf("- %s: %s\n\n", p2.Name, p2.Brand)
	
	result += "📦 **สถานะสินค้า:**\n"
	result += fmt.Sprintf("- %s: %s\n", p1.Name, map[bool]string{true: "มีสินค้า", false: "สินค้าหมด"}[p1.InStock])
	result += fmt.Sprintf("- %s: %s\n\n", p2.Name, map[bool]string{true: "มีสินค้า", false: "สินค้าหมด"}[p2.InStock])
	
	// Price comparison
	if p1.Price < p2.Price {
		result += fmt.Sprintf("💡 **คำแนะนำ:** %s ราคาถูกกว่า %.0f บาท\n", p1.Name, p2.Price-p1.Price)
	} else if p2.Price < p1.Price {
		result += fmt.Sprintf("💡 **คำแนะนำ:** %s ราคาถูกกว่า %.0f บาท\n", p2.Name, p1.Price-p2.Price)
	} else {
		result += "💡 **คำแนะนำ:** สินค้าทั้งสองราคาเท่ากัน\n"
	}

	return result, nil
}

// generateResponseWithTools generates final response combining tool results
func (t *ToolsNode) generateResponseWithTools(input core.NodeInput, toolResults string, executedTools []string) string {
	if input.NLUResult == nil {
		return "ขออภัยครับ มีปัญหาในการประมวลผล"
	}

	intent := input.NLUResult.PrimaryIntent
	baseResponse := ""

	switch intent {
	case "ask_price":
		baseResponse = "ขอแสดงข้อมูลราคาสินค้าให้นะครับ\n\n"
	case "purchase_intent":
		baseResponse = "นี่คือสินค้าที่เราแนะนำครับ\n\n"
	case "compare_product":
		baseResponse = "ขอเปรียบเทียบสินค้าให้ดูครับ\n\n"
	case "inquiry_intent":
		baseResponse = "ขอแสดงข้อมูลสินค้าครับ\n\n"
	default:
		baseResponse = "ข้อมูลสินค้าที่พบครับ\n\n"
	}

	finalResponse := baseResponse + toolResults + "\n\n"
	finalResponse += "มีอะไรให้ช่วยเหลือเพิ่มเติมไหมครับ?"

	return finalResponse
}