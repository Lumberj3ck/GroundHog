package patterns

// Pre-defined analysis patterns for the agent.

const (
	// PlanDay asks the agent to plan the day based on journal notes.
	PlanDay = "Based on the provided notes, create a detailed plan for my day."

	// AnalyseMyDay asks the agent to analyze the day based on journal notes.
	AnalyseMyDay = "Based on the provided notes, analyze my day and give me feedback."

	// Summarize asks the agent to summarize the notes.
	Summarize = "Summarize the key points from the provided notes in a few sentences."

	// IdentifyKeyPeople asks the agent to identify key people mentioned in the notes.
	IdentifyKeyPeople = "List all the people mentioned in the provided notes."

	// ExtractActionItems asks the agent to extract action items from the notes.
	ExtractActionItems = "Extract all action items or tasks from the provided notes."
)

const DefaultPattern = 	"No pattern"
// AllPatterns is a map of pattern names to their corresponding prompt.
var AllPatterns = map[string]string{
	DefaultPattern:      "",
	"Plan Day":        PlanDay,
	"Analyse My Day":  AnalyseMyDay,
	"Summarize Notes": Summarize,
	"Identify People": IdentifyKeyPeople,
	"Extract Actions": ExtractActionItems,
}
