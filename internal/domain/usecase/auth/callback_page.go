package auth

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
)

type pageKind int

const (
	pageSignedIn pageKind = iota
	pageAuthorized
	pageOAuthError
	pageSignInAlreadyHandled
	pageOAuthAlreadyHandled
)

//go:embed callback_page.html
var callbackPageHTML string

var callbackTemplate = template.Must(template.New("callback").Parse(callbackPageHTML))

type pageText struct {
	heading, body, next string
}

type callbackCopy struct {
	note                 string
	signedIn             pageText
	authorized           pageText
	signInAlreadyHandled pageText
	oauthAlreadyHandled  pageText
	oauthErrors          map[string]pageText
}

var callbackCopies = map[string]callbackCopy{
	"en": {
		note: "This page comes from the Tetiva app on your computer, which is why the address is 127.0.0.1.",
		signedIn: pageText{
			"You’re signed in to Tetiva",
			"Go back to the app — it will finish signing you in within a few seconds.",
			"You can close this tab.",
		},
		authorized: pageText{
			"Authorization received",
			"Tetiva has the authorization code and is exchanging it for a token. " +
				"The result will appear on the Auth tab (Authorization for a collection).",
			"You can close this tab.",
		},
		signInAlreadyHandled: pageText{
			"This link has already been used",
			"Sign-in through it is already complete. Go back to Tetiva.",
			"You can close this tab.",
		},
		oauthAlreadyHandled: pageText{
			"This authorization link was already handled",
			"The result is in Tetiva.",
			"You can close this tab.",
		},
		oauthErrors: map[string]pageText{
			"access_denied": {
				"Access denied",
				"The sign-in at the provider was cancelled, or the provider refused access. No token was issued.",
				"To try again, go back to Tetiva and press Get\u00a0token on the Auth tab (Authorization for a collection).",
			},
			"invalid_scope": {
				"The provider rejected the scope",
				"The requested scope is unknown to the provider or not allowed for this client. No token was issued.",
				"Correct the Scope field on the Auth tab (Authorization for a collection) in Tetiva " +
					"and press Get\u00a0token again.",
			},
			"server_error": {
				"The provider ran into an error",
				"The authorization server couldn’t process the request. No token was issued.",
				"Go back to Tetiva and press Get\u00a0token again. If it keeps failing, it has to be fixed at the provider.",
			},
			"temporarily_unavailable": {
				"The provider is temporarily unavailable",
				"The authorization server is overloaded or down for maintenance. No token was issued.",
				"Wait a couple of minutes, then go back to Tetiva and press Get\u00a0token again.",
			},
			"": {
				"The provider returned an error",
				"Authorization failed and no token was issued. " +
					"The error code and description are on the Auth tab (Authorization for a collection) in Tetiva.",
				"Check the Client\u00a0ID, the Authorization\u00a0URL and the client’s settings at the provider, " +
					"then press Get\u00a0token again.",
			},
		},
	},
	"ru": {
		note: "Эту страницу показывает само приложение Tetiva на\u00a0вашем компьютере, поэтому в\u00a0адресе 127.0.0.1.",
		signedIn: pageText{
			"Вы вошли в\u00a0Tetiva",
			"Вернитесь в\u00a0приложение\u00a0— вход завершится там через пару секунд.",
			"Эту вкладку можно закрыть.",
		},
		authorized: pageText{
			"Авторизация получена",
			"Tetiva получила код авторизации и\u00a0сейчас обменяет его на\u00a0токен. " +
				"Результат появится на\u00a0вкладке Auth (у\u00a0коллекции\u00a0— «Авторизация»).",
			"Эту вкладку можно закрыть.",
		},
		signInAlreadyHandled: pageText{
			"Эта ссылка уже использована",
			"Вход по\u00a0ней уже завершён. Вернитесь в\u00a0Tetiva.",
			"Эту вкладку можно закрыть.",
		},
		oauthAlreadyHandled: pageText{
			"Эта ссылка авторизации уже обработана",
			"Результат\u00a0— в\u00a0Tetiva.",
			"Эту вкладку можно закрыть.",
		},
		oauthErrors: map[string]pageText{
			"access_denied": {
				"Доступ не\u00a0выдан",
				"Вход у\u00a0провайдера отменён, или провайдер отказал в\u00a0доступе. Токен не\u00a0получен.",
				"Чтобы попробовать ещё раз, вернитесь в\u00a0Tetiva и\u00a0нажмите Get\u00a0token " +
					"на\u00a0вкладке Auth (у\u00a0коллекции\u00a0— «Авторизация»).",
			},
			"invalid_scope": {
				"Провайдер не\u00a0принял scope",
				"Запрошенные права неизвестны провайдеру или недоступны этому клиенту. Токен не\u00a0получен.",
				"Исправьте поле Scope на\u00a0вкладке Auth (у\u00a0коллекции\u00a0— «Авторизация») в\u00a0Tetiva " +
					"и\u00a0снова нажмите Get\u00a0token.",
			},
			"server_error": {
				"Сбой на\u00a0стороне провайдера",
				"Сервер авторизации не\u00a0смог обработать запрос. Токен не\u00a0получен.",
				"Вернитесь в\u00a0Tetiva и\u00a0нажмите Get\u00a0token ещё раз. Если сбой повторяется, исправлять его нужно у\u00a0провайдера.",
			},
			"temporarily_unavailable": {
				"Провайдер временно недоступен",
				"Сервер авторизации перегружен или на\u00a0обслуживании. Токен не\u00a0получен.",
				"Подождите пару минут, вернитесь в\u00a0Tetiva и\u00a0снова нажмите Get\u00a0token.",
			},
			"": {
				"Провайдер вернул ошибку",
				"Авторизация не\u00a0прошла, токен не\u00a0получен. Код и\u00a0описание ошибки\u00a0— " +
					"на\u00a0вкладке Auth (у\u00a0коллекции\u00a0— «Авторизация») в\u00a0Tetiva.",
				"Проверьте Client\u00a0ID, Authorization\u00a0URL и\u00a0настройки клиента у\u00a0провайдера, затем снова нажмите Get\u00a0token.",
			},
		},
	},
}

type callbackView struct {
	Lang, Kind, Heading, Body, Next, Code, Note string
}

type pageKey struct {
	lang string
	kind pageKind
	code string
}

var callbackPages = renderCallbackPages()

func renderCallbackPages() map[pageKey][]byte {
	pages := map[pageKey][]byte{}
	for lang, c := range callbackCopies {
		add := func(kind pageKind, code, badge string, text pageText) {
			view := callbackView{
				Lang: lang, Kind: badge, Heading: text.heading, Body: text.body, Next: text.next,
				Code: code, Note: c.note,
			}
			var buf bytes.Buffer
			if err := callbackTemplate.Execute(&buf, view); err != nil {
				panic(fmt.Sprintf("auth.renderCallbackPages: %s page %d: %v", lang, kind, err))
			}
			pages[pageKey{lang: lang, kind: kind, code: code}] = buf.Bytes()
		}

		add(pageSignedIn, "", "ok", c.signedIn)
		add(pageAuthorized, "", "ok", c.authorized)
		add(pageSignInAlreadyHandled, "", "ok", c.signInAlreadyHandled)
		add(pageOAuthAlreadyHandled, "", "neutral", c.oauthAlreadyHandled)
		for code, text := range c.oauthErrors {
			add(pageOAuthError, code, "error", text)
		}
	}

	return pages
}

// renderCallbackPage shows only allow-listed error codes, never the provider's own text.
func renderCallbackPage(locale string, kind pageKind, oauthError string) []byte {
	lang := "en"
	if locale == "ru" {
		lang = "ru"
	}

	code := ""
	if _, known := callbackCopies[lang].oauthErrors[oauthError]; known && kind == pageOAuthError {
		code = oauthError
	}

	return callbackPages[pageKey{lang: lang, kind: kind, code: code}]
}
