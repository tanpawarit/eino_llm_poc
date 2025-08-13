package nodes

import (
	"testing"
)

func TestEinoTools(t *testing.T) {
	tools := GetTools()
	
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}
	
	// Test that all tools are not nil
	for _, tool := range tools {
		if tool == nil {
			t.Error("Tool should not be nil")
		}
	}
}

func TestProductSearchTool(t *testing.T) {
	tool := ProductSearchTool()
	if tool == nil {
		t.Error("ProductSearchTool should not be nil")
	}
}

func TestProductComparisonTool(t *testing.T) {
	tool := ProductComparisonTool()
	if tool == nil {
		t.Error("ProductComparisonTool should not be nil")
	}
}

func TestPriceLookupTool(t *testing.T) {
	tool := PriceLookupTool()
	if tool == nil {
		t.Error("PriceLookupTool should not be nil")
	}
}