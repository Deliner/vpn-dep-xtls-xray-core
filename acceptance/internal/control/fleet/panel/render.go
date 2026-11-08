package panel

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"net/http"
	"time"

	"vpn-script/internal/control/fleet/panel/session"
	"vpn-script/internal/control/fleet/registry"
)

//go:embed views/*.html views/*.css
var assets embed.FS

var templates = template.Must(template.New("panel").Funcs(template.FuncMap{
	"stamp": timestamp, "short": shortHash, "phase": phaseLabel,
	"ordinal": func(index int) int { return index + 1 },
}).ParseFS(assets, "views/*.html"))

func render(w http.ResponseWriter, name string, user access, data view, status int) {
	data.CSRF, data.Authenticated = user.csrf, user.token != ""
	var buffer bytes.Buffer
	if err := templates.ExecuteTemplate(&buffer, name, data); err != nil {
		http.Error(w, "Не удалось отобразить страницу", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buffer.Bytes())
}

func failure(w http.ResponseWriter, user access, err error) {
	status, message := http.StatusInternalServerError, "Операция не завершена. Проверьте состояние контроллера перед повтором."
	switch {
	case errors.Is(err, registry.ErrInvalid):
		status, message = 400, "Проверьте поля формы и формат файла. Конфигурация должна соответствовать правилам установщика."
	case errors.Is(err, registry.ErrConflict):
		status, message = 409, "Состояние изменилось или узел занят другим rollout. Обновите данные и подготовьте новый запрос."
	case errors.Is(err, registry.ErrNotFound):
		status, message = 404, "Узел, кандидат или rollout не найден."
	case errors.Is(err, registry.ErrRevoked):
		status, message = 403, "Доступ узла отозван."
	case errors.Is(err, session.ErrExpired):
		status, message = 410, "Сессия или просмотр запроса истекли. Войдите заново и загрузите исходный файл."
	case errors.Is(err, session.ErrBusy):
		status, message = 409, "Подтверждение уже выполняется или достигнут лимит временных запросов. Повторите позже."
	}
	render(w, "error", user, view{Title: "Операция не завершена", Error: message}, status)
}

func stylesheet(w http.ResponseWriter, r *http.Request) {
	raw, err := assets.ReadFile("views/style.css")
	if err != nil {
		http.Error(w, "stylesheet unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(raw)
}

func timestamp(value int64) string {
	if value <= 0 {
		return "—"
	}
	return time.Unix(value, 0).UTC().Format("02 Jan 2006, 15:04:05 UTC")
}

func shortHash(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	if value == "" {
		return "—"
	}
	return value
}

func phaseLabel(value string) string {
	labels := map[string]string{"running": "Выполняется", "succeeded": "Завершён", "failed": "Ошибка",
		"canceled": "Остановлен", "pending": "В очереди", "waiting": "Ожидает отчёта", "observing": "Наблюдение",
		"ready": "Готов", "idle": "Нет задания", "applying": "Применение"}
	if label, ok := labels[value]; ok {
		return label
	}
	return value
}
