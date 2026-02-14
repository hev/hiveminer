# Create Form Skill

Creates extraction forms for threadminer through interactive questions.

## Usage
```
/create-form
```

## Behavior

When invoked, guide the user through creating a form by asking:

1. **What are you trying to find or research on Reddit?**
   - This becomes the form title and description
   - Example: "Gift recommendations for 45 year old men"

2. **What specific information do you want to extract from threads?**
   - Ask for a list of information they want
   - Example: "product name, price, positive reviews, negative reviews, purchase links"

3. **For each piece of information, determine:**
   - Field ID (snake_case identifier)
   - Field type:
     - `string` for text values
     - `number` for numeric values
     - `boolean` for yes/no questions
     - `array` for lists of items
   - Question (what to ask Claude to extract)
   - Search hints (keywords that help find relevant content)

4. **Generate the form JSON and save to `forms/` directory**

## Output Format

Generate a form like this:

```json
{
  "title": "Form Title",
  "description": "What this form extracts",
  "search_hints": ["relevant", "keywords"],
  "fields": [
    {
      "id": "field_name",
      "type": "string",
      "question": "What specific question should be answered?",
      "required": true
    },
    {
      "id": "another_field",
      "type": "array",
      "question": "What list of items should be extracted?",
      "search_hints": ["keywords", "to", "find"]
    }
  ]
}
```

## Field Types

- **string**: Single text value (product name, price range, URL)
- **number**: Numeric value (rating, price in dollars, count)
- **boolean**: True/false (is recommended?, has issues?)
- **array**: List of items (positive comments, features, links)

## Example Session

User: `/create-form`

Claude: I'll help you create an extraction form. What are you trying to find or research on Reddit?

User: Best mechanical keyboards for programming

Claude: Great! What specific information do you want to extract from threads about mechanical keyboards?

User: keyboard name, price, switch type, good things people say, bad things people say

Claude: I'll create a form with those fields. Let me generate it...

*Creates forms/mechanical-keyboards.json*

```json
{
  "title": "Mechanical Keyboard Recommendations",
  "description": "Extract keyboard recommendations for programming from Reddit threads",
  "search_hints": ["mechanical keyboard", "programming", "typing", "switch"],
  "fields": [
    {
      "id": "keyboard_name",
      "type": "string",
      "question": "What keyboard model is being recommended?",
      "required": true
    },
    {
      "id": "price",
      "type": "string",
      "question": "What is the price or price range mentioned?"
    },
    {
      "id": "switch_type",
      "type": "string",
      "question": "What switch type is mentioned (Cherry MX, Gateron, etc.)?"
    },
    {
      "id": "positive_feedback",
      "type": "array",
      "question": "What positive things do people say about this keyboard?",
      "search_hints": ["love", "great", "recommend", "best", "amazing"]
    },
    {
      "id": "negative_feedback",
      "type": "array",
      "question": "What negative things or concerns do people mention?",
      "search_hints": ["but", "however", "issue", "problem", "wish"]
    }
  ]
}
```

Form saved to `forms/mechanical-keyboards.json`

You can now run:
```
threadminer run --form forms/mechanical-keyboards.json -q "best mechanical keyboard programming" -r MechanicalKeyboards --limit 10
```
