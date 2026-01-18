// Package policy implements policy enforcement for ModelGate.
// synonyms.go provides synonym expansion for prompt injection pattern detection.
// This allows detecting semantic variations like "discard previous instructions"
// even when only "ignore previous instructions" is in the base pattern list.
package policy

import (
	"strings"
)

// SynonymGroup represents a group of semantically equivalent words
type SynonymGroup struct {
	Canonical string   // Primary word used in base patterns
	Synonyms  []string // Alternative words that mean the same thing
	Context   string   // Context hint: "action", "temporal", "object", "extraction", "role"
}

// SecuritySynonyms contains synonym groups relevant to prompt injection detection
var SecuritySynonyms = []SynonymGroup{
	// === Action verbs for instruction manipulation ===
	{
		Canonical: "ignore",
		Context:   "action",
		Synonyms: []string{
			"disregard", "discard", "skip", "neglect", "overlook",
			"dismiss", "forget", "bypass", "omit", "drop",
			"abandon", "reject", "override", "suppress", "exclude",
		},
	},
	{
		Canonical: "forget",
		Context:   "action",
		Synonyms: []string{
			"discard", "erase", "delete", "remove", "clear",
			"wipe", "reset", "abandon", "drop",
		},
	},
	{
		Canonical: "override",
		Context:   "action",
		Synonyms: []string{
			"overwrite", "replace", "supersede", "bypass", "circumvent",
			"nullify", "cancel", "revoke", "undo", "negate",
		},
	},

	// === Temporal/positional references ===
	{
		Canonical: "previous",
		Context:   "temporal",
		Synonyms: []string{
			"prior", "earlier", "above", "preceding", "former",
			"past", "original", "initial", "existing", "current",
			"old", "first", "before",
		},
	},
	{
		Canonical: "all previous",
		Context:   "temporal",
		Synonyms: []string{
			"all prior", "all earlier", "everything above", "all preceding",
			"everything before", "all existing", "all original",
		},
	},

	// === Objects being targeted ===
	{
		Canonical: "instructions",
		Context:   "object",
		Synonyms: []string{
			"directives", "commands", "rules", "guidelines", "prompts",
			"orders", "directions", "programming", "training", "conditioning",
			"guidance", "mandate", "protocols", "constraints", "policies",
		},
	},
	{
		Canonical: "system prompt",
		Context:   "object",
		Synonyms: []string{
			"system message", "initial prompt", "original prompt",
			"base prompt", "core prompt", "hidden prompt", "secret prompt",
			"system instructions", "base instructions", "core instructions",
		},
	},

	// === Extraction/reveal verbs ===
	{
		Canonical: "reveal",
		Context:   "extraction",
		Synonyms: []string{
			"show", "display", "expose", "tell", "output", "print",
			"disclose", "divulge", "share", "leak", "dump", "give",
			"provide", "echo", "repeat", "recite", "state",
		},
	},
	{
		Canonical: "what are",
		Context:   "extraction",
		Synonyms: []string{
			"what were", "tell me", "show me", "give me",
			"list", "describe", "explain", "output", "display",
		},
	},

	// === Role manipulation verbs ===
	{
		Canonical: "pretend",
		Context:   "role",
		Synonyms: []string{
			"act", "behave", "roleplay", "simulate", "assume",
			"impersonate", "mimic", "pose", "become", "transform",
			"imagine", "suppose", "envision", "play",
		},
	},
	{
		Canonical: "you are now",
		Context:   "role",
		Synonyms: []string{
			"you're now", "you have become", "you will be", "you shall be",
			"you must be", "from now on you are", "starting now you are",
			"henceforth you are", "consider yourself",
		},
	},
	{
		Canonical: "assume the role",
		Context:   "role",
		Synonyms: []string{
			"take the role", "take on the role", "adopt the role",
			"enter the role", "play the role", "act as", "become",
		},
	},

	// === Jailbreak-specific terms ===
	{
		Canonical: "jailbreak",
		Context:   "jailbreak",
		Synonyms: []string{
			"jail break", "jail-break", "unlock", "liberate", "free",
			"unshackle", "unleash", "release",
		},
	},
	{
		Canonical: "no restrictions",
		Context:   "jailbreak",
		Synonyms: []string{
			"without restrictions", "remove restrictions", "lift restrictions",
			"disable restrictions", "unrestricted", "no limits", "unlimited",
			"no constraints", "unconstrained", "no rules", "rule-free",
		},
	},
	{
		Canonical: "bypass",
		Context:   "jailbreak",
		Synonyms: []string{
			"circumvent", "evade", "avoid", "skip", "get around",
			"work around", "sidestep", "dodge", "escape",
		},
	},
}

// synonymMap provides O(1) lookup for synonyms
var synonymMap map[string][]string

// reverseSynonymMap maps each synonym back to its canonical form
var reverseSynonymMap map[string]string

func init() {
	buildSynonymMaps()
}

// buildSynonymMaps creates lookup maps from SecuritySynonyms
func buildSynonymMaps() {
	synonymMap = make(map[string][]string)
	reverseSynonymMap = make(map[string]string)

	for _, group := range SecuritySynonyms {
		canonical := strings.ToLower(group.Canonical)
		synonymMap[canonical] = make([]string, len(group.Synonyms))
		for i, syn := range group.Synonyms {
			synLower := strings.ToLower(syn)
			synonymMap[canonical][i] = synLower
			reverseSynonymMap[synLower] = canonical
		}
	}
}

// GetSynonyms returns all synonyms for a canonical word
func GetSynonyms(word string) []string {
	return synonymMap[strings.ToLower(word)]
}

// GetCanonical returns the canonical form of a word (or the word itself if not a known synonym)
func GetCanonical(word string) string {
	lower := strings.ToLower(word)
	if canonical, ok := reverseSynonymMap[lower]; ok {
		return canonical
	}
	return lower
}

// ExpandPatternWithSynonyms generates all synonym variations of a pattern
// Example: "ignore previous instructions" →
//   ["ignore previous instructions", "discard previous instructions",
//    "skip previous instructions", "ignore prior instructions", ...]
//
// To prevent combinatorial explosion, we limit expansion:
// - Only expand words that have synonyms
// - Limit to maxVariants total variants
func ExpandPatternWithSynonyms(pattern string, maxVariants int) []string {
	words := strings.Fields(strings.ToLower(pattern))
	if len(words) == 0 {
		return []string{}
	}

	// Find which words have synonyms (both as canonical and as synonyms themselves)
	wordSynonyms := make([][]string, len(words))
	for i, word := range words {
		// Start with the original word
		options := []string{word}

		// Check if this word is a canonical form with synonyms
		if syns, ok := synonymMap[word]; ok {
			options = append(options, syns...)
		}

		// Also check if this word is itself a synonym of something
		// and add sibling synonyms
		if canonical, ok := reverseSynonymMap[word]; ok {
			// Add the canonical form
			if !containsStr(options, canonical) {
				options = append(options, canonical)
			}
			// Add sibling synonyms
			if sibSyns, ok := synonymMap[canonical]; ok {
				for _, sib := range sibSyns {
					if !containsStr(options, sib) {
						options = append(options, sib)
					}
				}
			}
		}

		wordSynonyms[i] = options
	}

	// Generate combinations
	variants := generateCombinations(wordSynonyms, maxVariants)

	// Convert back to strings
	result := make([]string, 0, len(variants))
	seen := make(map[string]bool)
	for _, combo := range variants {
		variant := strings.Join(combo, " ")
		if !seen[variant] {
			seen[variant] = true
			result = append(result, variant)
		}
	}

	return result
}

// containsStr checks if a slice contains a string
func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// generateCombinations generates all combinations of word synonyms
// Limited to maxCombos to prevent explosion
func generateCombinations(wordOptions [][]string, maxCombos int) [][]string {
	if len(wordOptions) == 0 {
		return [][]string{}
	}

	// Start with combinations of the first word
	result := make([][]string, 0, len(wordOptions[0]))
	for _, word := range wordOptions[0] {
		result = append(result, []string{word})
	}

	// Extend with each subsequent word
	for i := 1; i < len(wordOptions); i++ {
		newResult := make([][]string, 0, len(result)*len(wordOptions[i]))
		for _, existing := range result {
			for _, word := range wordOptions[i] {
				if len(newResult) >= maxCombos {
					return newResult
				}
				newCombo := make([]string, len(existing)+1)
				copy(newCombo, existing)
				newCombo[len(existing)] = word
				newResult = append(newResult, newCombo)
			}
		}
		result = newResult
	}

	return result
}

// ExpandPatternEntry expands a PatternEntry using synonyms
// Returns a new PatternEntry with expanded variants
func ExpandPatternEntry(entry PatternEntry, maxVariantsPerPattern int) PatternEntry {
	expanded := PatternEntry{
		Primary:  entry.Primary,
		Variants: make([]string, 0),
	}

	// Start with existing variants
	expanded.Variants = append(expanded.Variants, entry.Variants...)

	// Expand primary pattern
	primaryExpanded := ExpandPatternWithSynonyms(entry.Primary, maxVariantsPerPattern)
	for _, v := range primaryExpanded {
		if v != entry.Primary && !contains(expanded.Variants, v) {
			expanded.Variants = append(expanded.Variants, v)
		}
	}

	// Expand each existing variant
	for _, variant := range entry.Variants {
		variantExpanded := ExpandPatternWithSynonyms(variant, maxVariantsPerPattern/2)
		for _, v := range variantExpanded {
			if v != variant && !contains(expanded.Variants, v) {
				expanded.Variants = append(expanded.Variants, v)
			}
		}
	}

	return expanded
}

// ExpandPatternCategory expands all patterns in a category using synonyms
func ExpandPatternCategory(category PatternCategory, maxVariantsPerPattern int) PatternCategory {
	expanded := PatternCategory{
		Name:     category.Name,
		Patterns: make([]PatternEntry, len(category.Patterns)),
	}

	for i, entry := range category.Patterns {
		expanded.Patterns[i] = ExpandPatternEntry(entry, maxVariantsPerPattern)
	}

	return expanded
}

// contains checks if a slice contains a string
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// GetExpandedIgnoreInstructionPatterns returns expanded patterns for ignore instructions
func GetExpandedIgnoreInstructionPatterns() PatternCategory {
	base := GetIgnoreInstructionPatterns()
	return ExpandPatternCategory(base, 50) // Max 50 variants per pattern
}

// GetExpandedSystemPromptPatterns returns expanded patterns for system prompt extraction
func GetExpandedSystemPromptPatterns() PatternCategory {
	base := GetSystemPromptPatterns()
	return ExpandPatternCategory(base, 50)
}

// GetExpandedRoleConfusionPatterns returns expanded patterns for role confusion
func GetExpandedRoleConfusionPatterns() PatternCategory {
	base := GetRoleConfusionPatterns()
	return ExpandPatternCategory(base, 50)
}

// GetExpandedJailbreakPatterns returns expanded patterns for jailbreak
func GetExpandedJailbreakPatterns() PatternCategory {
	base := GetJailbreakPatterns()
	return ExpandPatternCategory(base, 50)
}

// GetExpandedToolCoercionPatterns returns expanded patterns for tool coercion
func GetExpandedToolCoercionPatterns() PatternCategory {
	base := GetToolCoercionPatterns()
	return ExpandPatternCategory(base, 30)
}

// GetExpandedEncodingEvasionPatterns returns expanded patterns for encoding evasion
func GetExpandedEncodingEvasionPatterns() PatternCategory {
	base := GetEncodingEvasionPatterns()
	return ExpandPatternCategory(base, 20)
}

