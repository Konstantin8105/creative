# Правила разработки для проекта creative

## 0. Платформа

Разработка ведётся на **Windows**. Все команды и пути должны быть совместимы с Git Bash / PowerShell.

## 1. Соответствие DeepSeek API

Все изменения должны соответствовать требованиям, описанным в [`deepseek.rules.md`](deepseek.rules.md). Особое внимание:

- **Thinking mode**: `temperature`, `top_p`, `presence_penalty`, `frequency_penalty` не отправлять
- **Reasoning effort**: только `"high"` или `"max"` (валидация с нормализацией)
- **`reasoning_content` при tool_calls**: обязательно передавать обратно в API во всех последующих запросах
- **Keep-alive**: корректно обрабатывать пустые строки и SSE комментарии (`: keep-alive`)
- **`max_tokens`**: хардкод `8192` в [`p_routerai.go`](p_routerai.go), не менять

## 2. Тестирование

Перед каждым коммитом прогонять:

```bash
go test -short ./...
go build ./cmd/chat/
```

Тестовые файлы (данные для тестов) должны лежать в папке [`testdata/`](testdata/).

## 3. Обработка ошибок

При любой ошибке от API (особенно 400 Bad Request) сохраняется дамп сообщений в файл `chat_error_dump_<timestamp>.json` для диагностики.

**Возможные причины ошибок:**
1. **Consecutive user messages** — два сообщения с ролью `user` подряд
2. **Orphaned tool results** — tool result без предшествующего tool_call
3. **Missing `reasoning_content`** — отсутствует `reasoning_content` при tool_calls
4. **Превышение `context_size`** — слишком длинная история сообщений

При возникновении необъяснимых ошибок — проверять дамп-файлы в корне проекта, затем удалять их.
