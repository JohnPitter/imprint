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
