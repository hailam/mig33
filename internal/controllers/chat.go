package controllers

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/hailam/mig33/internal/adapters/httpapi"
	"github.com/hailam/mig33/internal/chat"
)

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		http.Error(w, "JSON content type required", http.StatusUnsupportedMediaType)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, 96*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "One JSON object required", http.StatusBadRequest)
		return false
	}
	return true
}

func (c *Controller) GetChatSession(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	result, err := c.sessions.Session(r.Context(), r.Header.Get("sid"))
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, httpapi.ChatSession{Username: result.Username, CsrfToken: result.CSRF})
}

func (c *Controller) GetChatGroups(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	groups, err := c.chat.ChatGroups(r.Context(), r.Header.Get("sid"))
	if err != nil {
		writeError(w, err)
		return
	}

	result := httpapi.ChatGroups{Groups: make([]httpapi.ChatGroup, 0, len(groups))}
	for _, group := range groups {
		result.Groups = append(result.Groups, httpapi.ChatGroup{Id: group.ID, Creator: group.Creator, Members: group.Members})
	}
	writeJSON(w, http.StatusOK, result)
}

func pageQuery(after, limit *int64) chat.PageQuery {
	result := chat.PageQuery{Limit: 50}
	if after != nil {
		result.AfterID = *after
	}
	if limit != nil {
		// Clamp values outside the supported window before converting to int,
		// including when this adapter is built on a 32-bit platform.
		if *limit < 1 || *limit > 200 {
			result.Limit = 0
		} else {
			result.Limit = int(*limit)
		}
	}
	return result
}

func (c *Controller) GetChatHistory(w http.ResponseWriter, r *http.Request, params httpapi.GetChatHistoryParams) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	result, err := c.chat.ChatHistory(r.Context(), r.Header.Get("sid"), chat.HistoryQuery{
		PageQuery: pageQuery(params.AfterId, params.Limit),
		Kind:      string(params.Kind),
		Target:    params.Target,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, httpapi.HistoryPage{
		Messages:      messageDTOs(result.Messages),
		NextAfterId:   result.NextAfterID,
		HasMore:       result.HasMore,
		ReadThroughId: result.ReadThroughID,
	})
}

func (c *Controller) SyncChat(w http.ResponseWriter, r *http.Request, params httpapi.SyncChatParams) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	result, err := c.chat.SyncChat(r.Context(), r.Header.Get("sid"), pageQuery(params.AfterId, params.Limit))
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, httpapi.MessagePage{
		Messages:    messageDTOs(result.Messages),
		NextAfterId: result.NextAfterID,
		HasMore:     result.HasMore,
	})
}

func (c *Controller) SendMessage(w http.ResponseWriter, r *http.Request, params httpapi.SendMessageParams) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body httpapi.SendMessageJSONRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}

	input := chat.MessageInput{
		Kind:    string(body.Kind),
		Target:  body.Target,
		Body:    body.Body,
		Color:   body.Color,
		PhotoID: body.PhotoId,
		Markup:  body.Markup,
	}
	result, err := c.chat.SendChatMessage(r.Context(), r.Header.Get("sid"), params.XCSRFToken, params.IdempotencyKey, input)
	if err != nil {
		writeError(w, err)
		return
	}

	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, httpapi.MessageResult{Message: messageDTO(result.Message), Replayed: result.Replayed})
}

func (c *Controller) MarkChatRead(w http.ResponseWriter, r *http.Request, params httpapi.MarkChatReadParams) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body httpapi.MarkChatReadJSONRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}

	id, err := c.chat.MarkChatRead(r.Context(), r.Header.Get("sid"), params.XCSRFToken, chat.ReadInput{
		Kind:      string(body.Kind),
		Target:    body.Target,
		ThroughID: body.ThroughId,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, httpapi.ReadPosition{ReadThroughId: id})
}

// ServeChatAPI covers methods that generated routing does not register and
// binding failures. It retains authentication-before-query-validation behavior.
func (c *Controller) ServeChatAPI(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/chat/session":
		c.GetChatSession(w, r)
	case "/api/chat/groups":
		c.GetChatGroups(w, r)
	case "/api/chat/messages":
		c.SendMessage(w, r, httpapi.SendMessageParams{XCSRFToken: r.Header.Get("X-CSRF-Token"), IdempotencyKey: r.Header.Get("Idempotency-Key")})
	case "/api/chat/read":
		c.MarkChatRead(w, r, httpapi.MarkChatReadParams{XCSRFToken: r.Header.Get("X-CSRF-Token")})
	case "/api/chat/history", "/api/chat/sync":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		if _, err := c.sessions.Session(r.Context(), r.Header.Get("sid")); err != nil {
			writeError(w, err)
			return
		}

		query := r.URL.Query()
		kind := query.Get("kind")
		if r.URL.Path == "/api/chat/history" && kind != "private" && kind != "group" && kind != "room" {
			http.Error(w, "kind must be private, group or room", http.StatusBadRequest)
			return
		}
		after, limit := int64(0), int64(50)
		var err error
		if value := query.Get("after_id"); value != "" {
			after, err = strconv.ParseInt(value, 10, 64)
		}
		if err != nil || after < 0 {
			http.Error(w, "Invalid after_id", http.StatusBadRequest)
			return
		}
		if value := query.Get("limit"); value != "" {
			limit, err = strconv.ParseInt(value, 10, 64)
		}
		if r.URL.Path == "/api/chat/history" {
			if err != nil || limit < 1 || limit > 200 || query.Get("target") == "" {
				http.Error(w, "target is required; limit must be 1 to 200", http.StatusBadRequest)
				return
			}
			c.GetChatHistory(w, r, httpapi.GetChatHistoryParams{
				Kind:    httpapi.ConversationKind(kind),
				Target:  query.Get("target"),
				AfterId: &after,
				Limit:   &limit,
			})
		} else {
			if err != nil || limit < 1 || limit > 200 {
				http.Error(w, "limit must be 1 to 200", http.StatusBadRequest)
				return
			}
			c.SyncChat(w, r, httpapi.SyncChatParams{AfterId: &after, Limit: &limit})
		}
	default:
		http.NotFound(w, r)
	}
}

func messageDTO(message chat.Message) httpapi.Message {
	result := httpapi.Message{
		Id:           message.ID,
		Kind:         httpapi.MessageKind(message.Kind),
		Conversation: message.Conversation,
		Sender:       message.Sender,
		Body:         message.Body,
		CreatedAt:    message.CreatedAt,
		Markup:       optionalString(message.Markup),
	}
	if message.Action {
		value := true
		result.Action = &value
	}
	if message.PhotoID != 0 {
		value := message.PhotoID
		result.PhotoId = &value
	}
	if message.Color != nil {
		value := int64(*message.Color)
		result.Color = &value
	}
	return result
}

func messageDTOs(messages []chat.Message) []httpapi.Message {
	result := make([]httpapi.Message, 0, len(messages))
	for _, message := range messages {
		result = append(result, messageDTO(message))
	}
	return result
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
