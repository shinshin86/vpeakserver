package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/shinshin86/vpeak"
)

const defaultWordType = "PROPER_NOUN"

var wordTypeToPos = map[string]string{
	"PROPER_NOUN":        "Japanese_Koyuumeishi_ippan",
	"LOCATION_NAME":      "Japanese_Koyuumeishi_place",
	"ORGANIZATION_NAME":  "Japanese_Koyuumeishi_ippan",
	"PERSON_NAME":        "Japanese_Koyuumeishi_jinmei",
	"PERSON_FAMILY_NAME": "Japanese_Koyuumeishi_sei",
	"PERSON_GIVEN_NAME":  "Japanese_Koyuumeishi_mei",
	"COMMON_NOUN":        "Japanese_Futsuu_meishi",
	"VERB":               "Japanese_Futsuu_meishi",
	"ADJECTIVE":          "Japanese_Futsuu_meishi",
	"SUFFIX":             "Japanese_Futsuu_meishi",
}

var posToWordType = map[string]string{
	"Japanese_Koyuumeishi_ippan":  "PROPER_NOUN",
	"Japanese_Koyuumeishi_place":  "LOCATION_NAME",
	"Japanese_Koyuumeishi_jinmei": "PERSON_NAME",
	"Japanese_Koyuumeishi_sei":    "PERSON_FAMILY_NAME",
	"Japanese_Koyuumeishi_mei":    "PERSON_GIVEN_NAME",
	"Japanese_Futsuu_meishi":      "COMMON_NOUN",
}

type UserDictWord struct {
	Surface       string `json:"surface"`
	Pronunciation string `json:"pronunciation"`
	Priority      int    `json:"priority"`
	AccentType    int    `json:"accent_type"`
	Pos           string `json:"pos"`
	Lang          string `json:"lang"`
	WordType      string `json:"word_type,omitempty"`
}

func parseUserDictWordRequest(values url.Values) (vpeak.DictEntry, error) {
	surface := strings.TrimSpace(values.Get("surface"))
	if surface == "" {
		return vpeak.DictEntry{}, fmt.Errorf("surface is required")
	}

	pronunciation := strings.TrimSpace(values.Get("pronunciation"))
	if pronunciation == "" {
		return vpeak.DictEntry{}, fmt.Errorf("pronunciation is required")
	}

	accentTypeRaw := strings.TrimSpace(values.Get("accent_type"))
	if accentTypeRaw == "" {
		return vpeak.DictEntry{}, fmt.Errorf("accent_type is required")
	}

	accentType, err := strconv.Atoi(accentTypeRaw)
	if err != nil {
		return vpeak.DictEntry{}, fmt.Errorf("accent_type must be an integer")
	}

	priority := 5
	if priorityRaw := strings.TrimSpace(values.Get("priority")); priorityRaw != "" {
		priority, err = strconv.Atoi(priorityRaw)
		if err != nil {
			return vpeak.DictEntry{}, fmt.Errorf("priority must be an integer")
		}
	}

	pos := strings.TrimSpace(values.Get("pos"))
	if pos == "" {
		wordType := strings.TrimSpace(values.Get("word_type"))
		if wordType == "" {
			wordType = defaultWordType
		}

		mappedPos, ok := wordTypeToPos[wordType]
		if !ok {
			return vpeak.DictEntry{}, fmt.Errorf("word_type is not supported")
		}
		pos = mappedPos
	}

	entry := vpeak.DictEntry{
		Surface:       surface,
		Pronunciation: pronunciation,
		Pos:           pos,
		Priority:      priority,
		AccentType:    accentType,
		Lang:          strings.TrimSpace(values.Get("lang")),
	}

	return vpeak.NormalizeDictEntry(entry)
}

func userDictWordFromEntry(entry vpeak.DictEntry) UserDictWord {
	return UserDictWord{
		Surface:       entry.Surface,
		Pronunciation: entry.Pronunciation,
		Priority:      entry.Priority,
		AccentType:    entry.AccentType,
		Pos:           entry.Pos,
		Lang:          entry.Lang,
		WordType:      posToWordType[entry.Pos],
	}
}
