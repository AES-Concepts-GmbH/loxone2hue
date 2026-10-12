package api

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/sbeyeler/loxone2hue/internal/config"
	"github.com/sbeyeler/loxone2hue/internal/models"
)

// xmlAttr escapes a string for safe use in an XML attribute value.
// Non-ASCII characters (e.g. umlauts) are passed through as raw UTF-8,
// relying on the UTF-8 BOM and XML encoding declaration.
func xmlAttr(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '&':
			b.WriteString("&amp;")
		case r == '"':
			b.WriteString("&quot;")
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sanitizeName creates a Loxone-safe identifier from a device name
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, "ä", "ae")
	name = strings.ReplaceAll(name, "ö", "oe")
	name = strings.ReplaceAll(name, "ü", "ue")
	name = strings.ReplaceAll(name, "ß", "ss")
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

var moodPattern = regexp.MustCompile(`^(.+)_mood_(\d+)$`)

// ExportVirtualInputs generates a Loxone Virtual UDP Input XML template.
// This creates command recognition entries matching the UDP feedback format.
// Query params:
//   - all=true: include all HUE lights, not just mapped ones
func (h *Handlers) ExportVirtualInputs(w http.ResponseWriter, r *http.Request) {
	// Use port from first miniserver, or default to 7777
	port := 7777
	if miniservers := config.GetLoxone().Miniservers; len(miniservers) > 0 && miniservers[0].Port > 0 {
		port = miniservers[0].Port
	}

	type inputDevice struct {
		Name     string
		LoxoneID string
	}

	var devices []inputDevice

	if r.URL.Query().Get("all") == "true" {
		// Include all HUE lights (use mapping if exists, otherwise generate ID)
		lights, err := h.hueClient.GetLights()
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, light := range lights {
			mappings := h.mappingManager.GetAllByHueID(light.ID)
			if len(mappings) == 0 {
				devices = append(devices, inputDevice{
					Name:     light.Name,
					LoxoneID: sanitizeName(light.Name),
				})
				continue
			}
			// A light may be wired to several Loxone IDs; each needs its own
			// set of command recognition entries.
			for _, mapping := range mappings {
				if mapping.HueType == "scene" {
					continue
				}
				devices = append(devices, inputDevice{
					Name:     mapping.Name,
					LoxoneID: mapping.LoxoneID,
				})
			}
		}
	} else {
		// Only mapped lights and groups (not scenes or mood mappings)
		mappings := config.GetMappings()
		for _, m := range mappings {
			if !m.Enabled || m.HueType == "scene" || moodPattern.MatchString(m.LoxoneID) {
				continue
			}
			devices = append(devices, inputDevice{
				Name:     m.Name,
				LoxoneID: m.LoxoneID,
			})
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].LoxoneID < devices[j].LoxoneID
	})

	nl := "\r\n" // CRLF required by Loxone Config (Windows)

	var buf bytes.Buffer
	// UTF-8 BOM required by Loxone Config
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	buf.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>" + nl)
	fmt.Fprintf(&buf, "<VirtualInUdp Title=\"Loxone2HUE Status\" Comment=\"\" Address=\"\" Port=\"%d\">%s", port, nl)
	buf.WriteString("\t<Info templateType=\"1\" minVersion=\"14051207\"/>" + nl)

	for _, d := range devices {
		name := xmlAttr(d.Name)
		id := xmlAttr(d.LoxoneID)

		fmt.Fprintf(&buf, "\t<VirtualInUdpCmd Title=\"%s On\" Comment=\"\" Address=\"\" Check=\"%s/on:\\v\" Signed=\"true\" Analog=\"true\" SourceValLow=\"0\" DestValLow=\"0\" SourceValHigh=\"1\" DestValHigh=\"1\" DefVal=\"0\" MinVal=\"0\" MaxVal=\"1\" Unit=\"&lt;v&gt;\" HintText=\"\"/>%s", name, id, nl)
		fmt.Fprintf(&buf, "\t<VirtualInUdpCmd Title=\"%s Helligkeit\" Comment=\"\" Address=\"\" Check=\"%s/bri:\\v\" Signed=\"true\" Analog=\"true\" SourceValLow=\"0\" DestValLow=\"0\" SourceValHigh=\"100\" DestValHigh=\"100\" DefVal=\"0\" MinVal=\"0\" MaxVal=\"100\" Unit=\"&lt;v&gt; %%\" HintText=\"\"/>%s", name, id, nl)
		fmt.Fprintf(&buf, "\t<VirtualInUdpCmd Title=\"%s Farbtemperatur\" Comment=\"\" Address=\"\" Check=\"%s/ct:\\v\" Signed=\"true\" Analog=\"true\" SourceValLow=\"153\" DestValLow=\"153\" SourceValHigh=\"500\" DestValHigh=\"500\" DefVal=\"0\" MinVal=\"153\" MaxVal=\"500\" Unit=\"&lt;v&gt;\" HintText=\"\"/>%s", name, id, nl)
		fmt.Fprintf(&buf, "\t<VirtualInUdpCmd Title=\"%s Farbe X\" Comment=\"\" Address=\"\" Check=\"%s/color_x:\\v\" Signed=\"true\" Analog=\"true\" SourceValLow=\"0\" DestValLow=\"0\" SourceValHigh=\"1\" DestValHigh=\"1\" DefVal=\"0\" MinVal=\"0\" MaxVal=\"1\" Unit=\"&lt;v&gt;\" HintText=\"\"/>%s", name, id, nl)
		fmt.Fprintf(&buf, "\t<VirtualInUdpCmd Title=\"%s Farbe Y\" Comment=\"\" Address=\"\" Check=\"%s/color_y:\\v\" Signed=\"true\" Analog=\"true\" SourceValLow=\"0\" DestValLow=\"0\" SourceValHigh=\"1\" DestValHigh=\"1\" DefVal=\"0\" MinVal=\"0\" MaxVal=\"1\" Unit=\"&lt;v&gt;\" HintText=\"\"/>%s", name, id, nl)
	}

	buf.WriteString("</VirtualInUdp>")

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"VIU_Loxone2HUE.xml\"")
	w.Write(buf.Bytes())
}

// ExportVirtualOutputs generates a Loxone Virtual HTTP Output XML template.
// This creates commands for controlling HUE devices from Loxone.
// Mood patterns (_mood_N) are grouped into a single MOOD command per base target.
func (h *Handlers) ExportVirtualOutputs(w http.ResponseWriter, r *http.Request) {
	// Determine gateway address using request hostname and configured server port
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if hostname, _, err := net.SplitHostPort(r.Host); err == nil {
		host = hostname
	}
	address := fmt.Sprintf("%s://%s:%d", scheme, host, config.GetServer().Port)

	mappings := config.GetMappings()

	// Detect mood base names and collect non-mood mappings
	type moodGroup struct {
		BaseName  string
		GroupName string // display name from the base group mapping
		Count     int    // number of scene moods (excluding mood_0)
	}
	moodGroups := make(map[string]*moodGroup)
	var directMappings []models.Mapping

	for _, m := range mappings {
		if !m.Enabled {
			continue
		}
		if matches := moodPattern.FindStringSubmatch(m.LoxoneID); matches != nil {
			baseName := matches[1]
			moodNum := matches[2]
			if mg, ok := moodGroups[baseName]; ok {
				if moodNum != "0" {
					mg.Count++
				}
			} else {
				count := 0
				if moodNum != "0" {
					count = 1
				}
				moodGroups[baseName] = &moodGroup{BaseName: baseName, Count: count}
			}
		} else {
			directMappings = append(directMappings, m)
			// If this mapping is a base group for mood entries, store its display name
			if mg, ok := moodGroups[m.LoxoneID]; ok {
				mg.GroupName = m.Name
			}
		}
	}

	// Second pass: resolve group names for mood groups that appeared before their base mapping
	for _, m := range directMappings {
		if mg, ok := moodGroups[m.LoxoneID]; ok && mg.GroupName == "" {
			mg.GroupName = m.Name
		}
	}

	// Sort mood base names
	moodBaseNames := make([]string, 0, len(moodGroups))
	for baseName := range moodGroups {
		moodBaseNames = append(moodBaseNames, baseName)
	}
	sort.Strings(moodBaseNames)

	nl := "\r\n" // CRLF required by Loxone Config (Windows)

	var buf bytes.Buffer
	// UTF-8 BOM required by Loxone Config
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	buf.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>" + nl)
	fmt.Fprintf(&buf, "<VirtualOut Title=\"Loxone2HUE\" Comment=\"\" Address=\"%s\" CmdInit=\"\" HintText=\"\" CloseAfterSend=\"true\" CmdSep=\";\">%s", xmlAttr(address), nl)
	buf.WriteString("\t<Info templateType=\"3\" minVersion=\"14051207\"/>" + nl)

	// Mood commands (0=Aus, 1-N=Szene)
	for _, baseName := range moodBaseNames {
		mg := moodGroups[baseName]
		title := mg.GroupName
		if title == "" {
			// Fallback: derive from LoxoneID
			title = strings.ReplaceAll(baseName, "_", " ")
			words := strings.Fields(title)
			for i, w := range words {
				if len(w) > 0 {
					words[i] = strings.ToUpper(w[:1]) + w[1:]
				}
			}
			title = strings.Join(words, " ")
		}

		comment := fmt.Sprintf("Mood: 0=Aus, 1-%d=Szene", mg.Count)
		fmt.Fprintf(&buf, "\t<VirtualOutCmd Title=\"%s (Mood)\" Comment=\"%s\" CmdOnMethod=\"GET\" CmdOn=\"/ws?cmd=MOOD %s &lt;v&gt;\" CmdOnHTTP=\"\" CmdOnPost=\"\" CmdOffMethod=\"GET\" CmdOff=\"\" CmdOffHTTP=\"\" CmdOffPost=\"\" CmdAnswer=\"\" HintText=\"\" Analog=\"true\" Repeat=\"0\" RepeatRate=\"0\"/>%s",
			xmlAttr(title), xmlAttr(comment), xmlAttr(baseName), nl)
	}

	// Direct light/group commands
	sort.Slice(directMappings, func(i, j int) bool {
		return directMappings[i].LoxoneID < directMappings[j].LoxoneID
	})

	for _, m := range directMappings {
		switch m.HueType {
		case "light", "group":
			// Analog dimmer: BRI 0 switches off, BRI > 0 switches on at that level.
			fmt.Fprintf(&buf, "\t<VirtualOutCmd Title=\"%s (Br)\" Comment=\"%s\" CmdOnMethod=\"GET\" CmdOn=\"/ws?cmd=SET %s BRI &lt;v&gt;\" CmdOnHTTP=\"\" CmdOnPost=\"\" CmdOffMethod=\"GET\" CmdOff=\"\" CmdOffHTTP=\"\" CmdOffPost=\"\" CmdAnswer=\"\" HintText=\"\" Analog=\"true\" Repeat=\"0\" RepeatRate=\"0\"/>%s",
				xmlAttr(m.Name), xmlAttr(m.Description), xmlAttr(m.LoxoneID), nl)
			// Digital switch for plain on/off wiring without a dimmer.
			fmt.Fprintf(&buf, "\t<VirtualOutCmd Title=\"%s (On/Off)\" Comment=\"%s\" CmdOnMethod=\"GET\" CmdOn=\"/ws?cmd=SET %s ON\" CmdOnHTTP=\"\" CmdOnPost=\"\" CmdOffMethod=\"GET\" CmdOff=\"/ws?cmd=SET %s OFF\" CmdOffHTTP=\"\" CmdOffPost=\"\" CmdAnswer=\"\" HintText=\"\" Analog=\"false\" Repeat=\"0\" RepeatRate=\"0\"/>%s",
				xmlAttr(m.Name), xmlAttr(m.Description), xmlAttr(m.LoxoneID), xmlAttr(m.LoxoneID), nl)
		}
	}

	buf.WriteString("</VirtualOut>")

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"VO_Loxone2HUE.xml\"")
	w.Write(buf.Bytes())
}
