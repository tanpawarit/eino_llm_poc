package services

import (
	"context"
	"strings"
)

// Product represents a simple product
type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Brand       string  `json:"brand"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	InStock     bool    `json:"in_stock"`
}

// ProductService handles product operations
type ProductService struct {
	products []Product
}

// NewProductService creates service with simple mock data
func NewProductService() *ProductService {
	return &ProductService{
		products: []Product{
			{
				ID:          "nb001",
				Name:        "MacBook Pro",
				Brand:       "Apple",
				Price:       59900,
				Description: "โน้ตบุ๊คสำหรับงานหนัก",
				InStock:     true,
			},
			{
				ID:          "nb002", 
				Name:        "ThinkPad X1",
				Brand:       "Lenovo",
				Price:       45900,
				Description: "โน้ตบุ๊คบิสเนส",
				InStock:     true,
			},
			{
				ID:          "pc001",
				Name:        "iMac 24-inch",
				Brand:       "Apple", 
				Price:       49900,
				Description: "คอมพิวเตอร์ตั้งโต๊ะ",
				InStock:     false,
			},
			{
				ID:          "acc001",
				Name:        "Magic Mouse",
				Brand:       "Apple",
				Price:       3500,
				Description: "เมาส์ไร้สาย",
				InStock:     true,
			},
		},
	}
}

// SearchProducts searches for products by query
func (ps *ProductService) SearchProducts(ctx context.Context, query string) []Product {
	if query == "" {
		return ps.products
	}
	
	var results []Product
	queryLower := strings.ToLower(query)
	
	for _, product := range ps.products {
		if strings.Contains(strings.ToLower(product.Name), queryLower) ||
		   strings.Contains(strings.ToLower(product.Brand), queryLower) ||
		   strings.Contains(strings.ToLower(product.Description), queryLower) {
			results = append(results, product)
		}
	}
	
	return results
}