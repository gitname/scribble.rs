package frontend

import (
	"log"
	"net/http"
	"strings"

	"github.com/scribble-rs/scribble.rs/internal/api"
	"github.com/scribble-rs/scribble.rs/internal/translations"
	"golang.org/x/text/language"
)

type lobbyPageData struct {
	*BasePageConfig
	*api.LobbyData

	Translation translations.Translation
	Locale      string
}

type robotPageData struct {
	*BasePageConfig
	*api.LobbyData
}

// ssrEnterLobby opens a lobby, either opening it directly or asking for a lobby.
func ssrEnterLobby(writer http.ResponseWriter, request *http.Request) {
	lobby, err := api.GetLobby(request)
	if err != nil {
		userFacingError(writer, err.Error())
		return
	}

	userAgent := strings.ToLower(request.UserAgent())
	if !(strings.Contains(userAgent, "gecko") || strings.Contains(userAgent, "chrome") || strings.Contains(userAgent, "opera") || strings.Contains(userAgent, "safari")) {
		templatingError := pageTemplates.ExecuteTemplate(writer, "robot-page", &robotPageData{
			BasePageConfig: currentBasePageConfig,
			LobbyData:      api.CreateLobbyData(lobby),
		})
		if templatingError != nil {
			log.Printf("error templating robot page: %d\n", templatingError)
		}
		return
	}

	translation, locale := determineTranslation(request)
	requestAddress := api.GetIPAddressFromRequest(request)

	var pageData *lobbyPageData
	lobby.Synchronized(func() {
		player := api.GetPlayer(lobby, request)

		if player == nil {
			if !lobby.HasFreePlayerSlot() {
				userFacingError(writer, "Sorry, but the lobby is full.")
				return
			}

			if !lobby.CanIPConnect(requestAddress) {
				userFacingError(writer, "Sorry, but you have exceeded the maximum number of clients per IP.")
				return
			}

			newPlayer := lobby.JoinPlayer(api.GetPlayername(request))

			api.SetUsersessionCookie(writer, newPlayer)
		} else {
			if player.Connected && player.GetWebsocket() != nil {
				userFacingError(writer, "It appears you already have an open tab for this lobby.")
				return
			}
			player.SetLastKnownAddress(requestAddress)
		}

		pageData = &lobbyPageData{
			BasePageConfig: currentBasePageConfig,
			LobbyData:      api.CreateLobbyData(lobby),
			Translation:    translation,
			Locale:         locale,
		}
	})

	// If the pagedata isn't initialized, it means the synchronized block has exited.
	// In this case we don't want to template the lobby, since an error has occurred
	// and probably already has been handled.
	if pageData != nil {
		templateError := pageTemplates.ExecuteTemplate(writer, "lobby-page", pageData)
		if templateError != nil {
			log.Printf("Error templating lobby: %s\n", templateError)
		}
	}
}

func determineTranslation(r *http.Request) (translations.Translation, string) {
	var translation translations.Translation

	languageTags, _, languageParseError := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if languageParseError == nil {
		for _, languageTag := range languageTags {
			fullLanguageIdentifier := languageTag.String()
			fullLanguageIdentifierLowercased := strings.ToLower(fullLanguageIdentifier)
			translation = translations.GetLanguage(fullLanguageIdentifierLowercased)
			if translation != nil {
				return translation, fullLanguageIdentifierLowercased
			}

			baseLanguageIdentifier, _ := languageTag.Base()
			baseLanguageIdentifierLowercased := strings.ToLower(baseLanguageIdentifier.String())
			translation = translations.GetLanguage(baseLanguageIdentifierLowercased)
			if translation != nil {
				return translation, baseLanguageIdentifierLowercased
			}
		}
	}

	return translations.DefaultTranslation, "en-us"
}
