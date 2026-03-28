# blowerboo

Минималистичный проект для оркестрации генеративного пайплайна.

## Текущее состояние

Сейчас в проекте есть два режима:

1. `Copilot Chat` с кастомными агентами в `.github/agents/`
2. `Go CLI` со stub-реализациями в `internal/`

Основной рабочий сценарий для VS Code Copilot Chat: `@pipeline`.

## Агенты Copilot

- `@pipeline` — актуальный агент. За один запуск выполняет этапы spec → planner → executor и создает:
  - `projects/{timestamp}/spec.md`
  - `projects/{timestamp}/plan.md`
  - `projects/{timestamp}/prompts.md`
- `@spec`, `@planner`, `@executor` — legacy-агенты для точечных правок и ручного пошагового прогона.

Важно: автопереключение между `@spec -> @planner -> @executor` в Copilot Chat не гарантируется, поэтому для обычной работы используйте `@pipeline`.

## Быстрый старт (Copilot Chat)

В чате VS Code:

```text
@pipeline <ваш сырой промпт>
```

## Быстрый старт (Go CLI)

```bash
go run ./cmd/blowerboo "ваш промпт"
```

CLI нужен для локальной проверки структуры пайплайна и работы stub-агентов.

## Структура проекта

```text
blowerboo/
├── .github/agents/        # Конфиги и инструкции Copilot-агентов
├── cmd/blowerboo/         # Точка входа CLI
├── internal/              # Stub-агенты, модели, оркестратор, провайдерный реестр
├── projects/              # Артефакты запусков (spec/plan/prompts)
└── docs/                  # Технические заметки
```

## Правила разработки

Основные правила и ограничения: [`.github/copilot-instructions.md`](./.github/copilot-instructions.md)
