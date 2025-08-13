package nodes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/components/tool"
	"eino_llm_poc/internal/services"
)

// ProductSearchTool creates a product search tool using Eino's InferTool
func ProductSearchTool() tool.BaseTool {
	tool, _ := utils.InferTool("product_database", "Search for products in the database based on query", 
		func(ctx context.Context, query string) (string, error) {
			log.Printf("🔍 Searching products for query: %s", query)
			
			productService := services.NewProductService()
			products := productService.SearchProducts(ctx, query)
			
			if len(products) == 0 {
				return "🛍️ ไม่พบสินค้าที่ตรงกับคำค้นหา", nil
			}

			var results []string
			results = append(results, "🛍️ สินค้าที่พบ:")
			
			for i, product := range products {
				if i >= 3 { // Limit to 3 products
					break
				}
				
				stockStatus := "✅ มีสินค้า"
				if !product.InStock {
					stockStatus = "❌ สินค้าหมด"
				}
				
				results = append(results, fmt.Sprintf("%d. **%s** (%s)", i+1, product.Name, product.Brand))
				results = append(results, fmt.Sprintf("   💰 ราคา: %.0f บาท", product.Price))
				results = append(results, fmt.Sprintf("   📝 %s", product.Description))
				results = append(results, fmt.Sprintf("   📦 สถานะ: %s", stockStatus))
			}

			return strings.Join(results, "\n"), nil
		})
	return tool
}

// ProductComparisonTool creates a product comparison tool using Eino's InferTool
func ProductComparisonTool() tool.BaseTool {
	tool, _ := utils.InferTool("comparison_engine", "Compare products based on price, brand, and availability",
		func(ctx context.Context, category string) (string, error) {
			log.Printf("🔍 Comparing products for category: %s", category)
			
			productService := services.NewProductService()
			products := productService.SearchProducts(ctx, category)
			
			if len(products) < 2 {
				return "⚠️ ต้องมีสินค้าอย่างน้อย 2 รายการเพื่อเปรียบเทียบ", nil
			}

			// Compare first 2 products
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
				result += fmt.Sprintf("💡 **คำแนะนำ:** %s ราคาถูกกว่า %.0f บาท", p1.Name, p2.Price-p1.Price)
			} else if p2.Price < p1.Price {
				result += fmt.Sprintf("💡 **คำแนะนำ:** %s ราคาถูกกว่า %.0f บาท", p2.Name, p1.Price-p2.Price)
			} else {
				result += "💡 **คำแนะนำ:** สินค้าทั้งสองราคาเท่ากัน"
			}

			return result, nil
		})
	return tool
}

// PriceLookupTool creates a price lookup tool using Eino's InferTool
func PriceLookupTool() tool.BaseTool {
	tool, _ := utils.InferTool("price_lookup", "Look up prices for specific products",
		func(ctx context.Context, productName string) (string, error) {
			log.Printf("💰 Looking up price for product: %s", productName)
			
			productService := services.NewProductService()
			products := productService.SearchProducts(ctx, productName)
			
			if len(products) == 0 {
				return fmt.Sprintf("❌ ไม่พบสินค้า '%s'", productName), nil
			}

			product := products[0]
			stockStatus := map[bool]string{true: "มีสินค้า", false: "สินค้าหมด"}[product.InStock]
			
			return fmt.Sprintf("📦 **%s** (%s): **%.0f บาท** - %s", 
				product.Name, product.Brand, product.Price, stockStatus), nil
		})
	return tool
}

// GetTools returns all available tools as BaseTool instances
func GetTools() []tool.BaseTool {
	return []tool.BaseTool{
		ProductSearchTool(),
		ProductComparisonTool(),
		PriceLookupTool(),
	}
}