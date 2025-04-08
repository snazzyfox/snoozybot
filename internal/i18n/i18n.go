package i18n

import (
	"bytes"
	"embed"
	"io/fs"
	"strings"
	"text/template"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

//go:embed metadata/*.yaml response/*.yaml
var files embed.FS

const defaultLang = dg.EnglishUS

var metadata = make(map[string]map[dg.Locale]string)               // flattened key -> locale -> value
var response = make(map[string]map[dg.Locale][]*template.Template) // flattened key -> locale -> templates
var fragments = make(map[string]map[dg.Locale]map[string][]string) // key -> locale -> fragment key -> values

type Vars map[string]interface{}
type AddFunc = func(lang dg.Locale, keyParts []string, value string)

func init() {
	for _, file := range getFiles("response") {
		loadFile("response", file, addResponse)
	}
	for _, file := range getFiles("metadata") {
		loadFile("metadata", file, addMetadata)
	}
	log.Debug().Msg("Language files loaded")
}

func getFiles(dirName string) []fs.DirEntry {
	dir, err := files.ReadDir(dirName)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to list embedded language files.")
	}
	return dir
}

func loadFile(path string, file fs.DirEntry, add AddFunc) {
	fileName := path + "/" + file.Name()
	log.Debug().Str("file", fileName).Msg("Loading language file.")
	content, err := files.ReadFile(fileName)
	if err != nil {
		log.Panic().Err(err).Str("file", fileName).Msg("Failed to open embedded language file.")
	}
	lang := dg.Locale(strings.Split(file.Name(), ".")[0])
	data := make(map[string]interface{})
	yaml.Unmarshal(content, &data)
	flattenAndAdd(lang, []string{}, &data, add)
}

func addResponse(lang dg.Locale, keyParts []string, value string) {
	idx := lo.IndexOf(keyParts, "fragments")
	if idx != -1 {
		key := strings.Join(keyParts[:idx], ".")
		fragKey := strings.Join(keyParts[idx+1:], ".")
		if fragments[key] == nil {
			fragments[key] = map[dg.Locale]map[string][]string{lang: {fragKey: {value}}}
		} else if fragments[key][lang] == nil {
			fragments[key][lang] = map[string][]string{fragKey: {value}}
		} else {
			fragments[key][lang][fragKey] = append(fragments[key][lang][fragKey], value)
		}
	} else {
		key := strings.Join(keyParts, ".")
		templ := template.Must(template.New(key).Option("missingkey=error").Parse(value))
		if response[key] == nil {
			response[key] = map[dg.Locale][]*template.Template{lang: {templ}}
		} else {
			response[key][lang] = append(response[key][lang], templ)
		}
	}
}

func addMetadata(lang dg.Locale, keyParts []string, value string) {
	key := strings.Join(keyParts, ".")
	if metadata[key] == nil {
		metadata[key] = map[dg.Locale]string{lang: value}
	} else {
		metadata[key][lang] = value
	}
}

func flattenAndAdd(lang dg.Locale, keyParts []string, data *map[string]interface{}, add AddFunc) {
	for key, value := range *data {
		tKey := append(keyParts, key)
		switch v := value.(type) {
		case string:
			log.Trace().Strs("key", tKey).Str("value", v).Msg("Adding translation")
			add(lang, tKey, v)
		case []interface{}:
			for _, item := range v {
				flattenAndAdd(lang, keyParts, &map[string]interface{}{key: item}, add)
			}
		case map[string]interface{}:
			flattenAndAdd(lang, tKey, &v, add)
		default:
			log.Warn().Any("value", v).Strs("key", tKey).Msg("Unknown data structure encountered when parsing language file")
		}
	}
}

func Get(lang dg.Locale, key string, vars ...*Vars) string {
	var opt *template.Template
	if opts := response[key][lang]; len(opts) == 0 {
		if lang != defaultLang {
			log.Warn().Str("key", key).Str("locale", string(lang)).Msg("Translation not available, falling back to default locale.")
			return Get(defaultLang, key)
		} else {
			log.Error().Str("key", key).Str("locale", string(lang)).Msg("String not available in any languages. Returning key as-is!")
			return key
		}
	} else if len(opts) == 1 {
		opt = opts[0]
	} else {
		opt = lo.Sample(opts)
	}
	return TemplateString(opt, vars...)
}

func GetFragments(lang dg.Locale, key string, vars ...*Vars) Vars {
	frags := fragments[key][lang]
	res := make(Vars)
	for k, v := range frags {
		res[k] = lo.Sample(v)
	}
	return res
}

func TemplateString(templ *template.Template, vars ...*Vars) string {
	var buf bytes.Buffer
	if len(vars) == 0 {
		vars = []*Vars{{}}
	}
	if err := templ.Execute(&buf, vars[0]); err != nil {
		log.Error().Any("template", *templ).Any("vars", vars).Msg("Failed to template string")
	}
	return buf.String()
}

func GetMetadata(key string) *map[dg.Locale]string {
	res := metadata[key]
	log.Trace().Str("key", key).Any("translations", res).Msg("Fetched localizations")
	return &res
}
