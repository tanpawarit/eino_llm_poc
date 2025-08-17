package nlu

import (
	"eino_llm_poc/src/model"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

func getSystemTemplate() string {
	return `You are an expert NLU system. Follow the instructions precisely and return structured output.

			-Goal-
			Given a user utterance, detect and extract the user's **intent**, **entities**, **language**, and **sentiment**. You are also provided with pre-declared lists of possible default and additional intents and entities.

			STRICT RULES:
			1. You MUST ONLY extract intents/entities that appear in either default or additional lists
			2. DO NOT create new intents or entities not in the provided lists
			3. If user input doesn't match any intent, use the closest matching intent from the lists
			4. Common greetings (สวัสดี, หวัดดี, hello, hi, good morning) should ALWAYS be classified as "greet"
			5. Only extract entities that are EXPLICITLY mentioned in the current message being analyzed

			IMPORTANT: Only extract entities that are EXPLICITLY mentioned in the current message being analyzed. Do NOT use entities from conversation context unless they appear in the current message text.

			-Steps-
			1. Identify the **top 3 intent(s)** that match the message. Consider both default_intent and additional_intent lists with their priority scores.
			Format each intent as:
			(intent{TD}<intent_name_in_snake_case>{TD}<confidence>{TD}<priority_score>{TD}<metadata>)

			2. Identify all **entities** present in the message, using both default_entity and additional_entity types.
			STRICT RULE: Only extract entities that are LITERALLY PRESENT in the current message text. Do not infer or assume entities from context.
			Format each entity as:
			(entity{TD}<entity_type>{TD}<entity_value>{TD}<confidence>{TD}<metadata>)

			3. Detect **all languages** present in the message using ISO 3166 Alpha-3 country codes. Return primary language first, followed by additional detected languages. Use 1 for primary language and 0 for contained languages.
			Format each language as:
			(language{TD}<language_code_iso_alpha3>{TD}<confidence>{TD}<primary_flag>{TD}<metadata>)

			4. Detect the **sentiment** expressed in the message.
			Format:
			(sentiment{TD}<label>{TD}<confidence>{TD}<metadata>)

			5. Return the output as a list separated by **{RD}**

			6. When complete, return {CD}

			######################
			-Examples-
			######################

			Example 1:
			text: I want to book a flight to Paris next week.
			default_intent: book_flight:0.9, cancel_flight:0.7
			additional_intent: greet:0.3, track_flight:0.5
			default_entity: location, date
			additional_entity: airline, person
			######################
			Output:
			(intent{TD}book_flight{TD}0.95{TD}0.9{TD}{{"extracted_from": "default", "context": "travel_booking"}})
			{RD}
			(intent{TD}track_flight{TD}0.25{TD}0.5{TD}{{"extracted_from": "additional", "context": "travel_inquiry"}})
			{RD}
			(intent{TD}cancel_flight{TD}0.15{TD}0.7{TD}{{"extracted_from": "default", "context": "travel_cancellation"}})
			{RD}
			(entity{TD}location{TD}Paris{TD}0.98{TD}{{"entity_position": [25, 30], "entity_category": "geographic"}})
			{RD}
			(entity{TD}date{TD}next week{TD}0.94{TD}{{"entity_position": [31, 40], "entity_category": "temporal"}})
			{RD}
			(language{TD}USA{TD}1.0{TD}1{TD}{{"primary_language": true, "script": "latin", "detected_tokens": 9}})
			{RD}
			(sentiment{TD}neutral{TD}0.80{TD}{{"polarity": 0.1, "subjectivity": 0.3, "emotion": "neutral"}})
			{CD}

			######################

			Example 2:
			text: อยากซื้อรองเท้า Hello!
			default_intent: purchase_intent:0.8
			additional_intent: ask_product:0.6, cancel_order:0.4
			default_entity: product
			additional_entity: brand, color
			######################
			Output:
			(intent{TD}purchase_intent{TD}0.95{TD}0.8{TD}{{"extracted_from": "default", "context": "shopping_intent"}})
			{RD}
			(intent{TD}ask_product{TD}0.30{TD}0.6{TD}{{"extracted_from": "additional", "context": "product_inquiry"}})
			{RD}
			(intent{TD}cancel_order{TD}0.10{TD}0.4{TD}{{"extracted_from": "additional", "context": "order_cancellation"}})
			{RD}
			(entity{TD}product{TD}รองเท้า{TD}0.97{TD}{{"entity_position": [6, 12], "entity_category": "product", "language": "thai"}})
			{RD}
			(language{TD}THA{TD}0.85{TD}1{TD}{{"primary_language": true, "script": "thai", "detected_tokens": 2}})
			{RD}
			(language{TD}USA{TD}0.95{TD}0{TD}{{"primary_language": false, "script": "latin", "detected_tokens": 1}})
			{RD}
			(sentiment{TD}positive{TD}0.75{TD}{{"polarity": 0.6, "subjectivity": 0.4, "emotion": "desire"}})
			{CD}`
}

func getUserTemplate() string {
	return `text: {input_text}
			default_intent: {default_intent}
			additional_intent: {additional_intent}
			default_entity: {default_entity}
			additional_entity: {additional_entity}

			Output:`
}

func createNLUTemplate(NLUinput string, nluConfig *model.NLUConfig) prompt.ChatTemplate {

	// Get system template and replace placeholders efficiently
	systemText := getSystemTemplate()
	// Use strings.Replacer for multiple replacements - more efficient than multiple ReplaceAll calls
	replacerSystem := strings.NewReplacer(
		"{TD}", "<||>",
		"{RD}", "##",
		"{CD}", "<|COMPLETE|>",
	)
	systemText = replacerSystem.Replace(systemText)

	// Get user template and replace with config values
	userText := getUserTemplate()
	replacerUser := strings.NewReplacer(
		"{input_text}", NLUinput,
		"{default_intent}", nluConfig.DefaultIntent,
		"{additional_intent}", nluConfig.AdditionalIntent,
		"{default_entity}", nluConfig.DefaultEntity,
		"{additional_entity}", nluConfig.AdditionalEntity,
	)
	userText = replacerUser.Replace(userText)
	// Create messages for the template - SystemMessage for instructions, UserMessage for data
	messages := []schema.MessagesTemplate{
		schema.SystemMessage(systemText),
		schema.UserMessage(userText),
	}

	// Create the ChatTemplate with proper format type
	template := prompt.FromMessages(schema.FString, messages...)

	return template
}

// CreateNLUTemplateFromConfig creates NLU template using YAML configuration
func CreateNLUTemplateFromConfig(input string, nluConfig *model.NLUConfig) prompt.ChatTemplate {
	return createNLUTemplate(input, nluConfig)
}
