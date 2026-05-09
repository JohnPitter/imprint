package pipeline

const compressSystemPrompt = `You are an AI observation compressor. Given a raw tool observation from a coding session, extract the essential information.

Respond with XML in this exact format:
<observation>
  <type>file_operation|command_execution|search|conversation|error|decision|discovery|other</type>
  <title>Short descriptive title (max 80 chars)</title>
  <subtitle>Optional one-line subtitle</subtitle>
  <narrative>2-3 sentence summary of what happened and why it matters</narrative>
  <facts>
    <fact>Key fact 1</fact>
    <fact>Key fact 2</fact>
  </facts>
  <concepts>
    <concept>concept1</concept>
    <concept>concept2</concept>
  </concepts>
  <files>
    <file>path/to/file.ts</file>
  </files>
  <importance>1-10 score (10=critical decision, 1=trivial)</importance>
</observation>`

const compressUserPrompt = `Compress this observation:
Tool: %s
Input: %s
Output: %s`

// compressSystemPromptHybrid is used when IMPRINT_EXTRACTION_MODE=hybrid (the
// default). The Go regex pre-pass already populates files and most concepts,
// so the LLM only owns the parts that need real understanding: type, title,
// subtitle, narrative, importance, and any conceptual idea the regex missed.
// Smaller response, fewer tokens, same essential information.
const compressSystemPromptHybrid = `You are an AI observation compressor. A regex pre-pass already extracted the file paths and obvious entity names from this observation; your job is to add the parts that need understanding.

Respond with XML in this exact format:
<observation>
  <type>file_operation|command_execution|search|conversation|error|decision|discovery|other</type>
  <title>Short descriptive title (max 80 chars)</title>
  <subtitle>Optional one-line subtitle</subtitle>
  <narrative>2-3 sentence summary of what happened and why it matters</narrative>
  <facts>
    <fact>Key fact about the observation</fact>
  </facts>
  <concepts>
    <concept>Higher-level concept the regex couldn't infer (skip plain class names — those are already captured)</concept>
  </concepts>
  <importance>1-10 score (10=critical decision, 1=trivial)</importance>
</observation>

Files are extracted automatically; do NOT repeat them. Concepts here should be domain ideas (e.g. "OAuth refresh flow", "rate limiting"), not class/function names.`

const summarizeSystemPrompt = `You are a session summarizer. Given a list of compressed observations from a coding session, create a concise session summary.

Respond with XML:
<summary>
  <title>Brief session title (max 100 chars)</title>
  <narrative>2-4 sentence narrative of what was accomplished</narrative>
  <key_decisions>
    <decision>Decision 1</decision>
  </key_decisions>
  <files_modified>
    <file>path/to/file</file>
  </files_modified>
  <concepts>
    <concept>concept</concept>
  </concepts>
</summary>`

const summarizeUserPrompt = `Summarize this session with %d observations:

%s`
