# Frontend часть
https://github.com/Donal-Noye/filetransfer-frontend

# FileShare

Простой файлообменник с ограниченным сроком жизни файлов.

## Возможности
- Загрузка одного или нескольких файлов за раз
- Батч-ссылка на группу файлов
- Скачивание всего батча как zip
- Настраиваемый срок жизни (1 день / 3 дня / неделя)
- Автоматическая очистка просроченных файлов
- Проверка типа и размера файла

## Запуск

\`\`\`bash
docker compose up --build
\`\`\`

Сервер поднимется на `http://localhost:8080`.

## API

### Загрузить файлы
\`\`\`bash
curl -F "myFile=@photo.jpg" -F "myFile=@doc.pdf" -F "expires=3d" http://localhost:8080/upload
\`\`\`
Ответ: `{"link": "http://localhost:8080/batch/{id}"}`

### Получить список файлов батча
\`\`\`bash
curl http://localhost:8080/batch/{id}
\`\`\`

### Скачать один файл
\`\`\`bash
curl -O -J http://localhost:8080/files/{id}
\`\`\`

### Скачать весь батч архивом
\`\`\`bash
curl -O -J http://localhost:8080/batch/{id}/zip
\`\`\`

## Технические детали
- Стандартная библиотека `net/http` без сторонних роутеров (Go 1.22+ path patterns)
- In-memory хранилище метаданных, защищённое `sync.RWMutex`
- Фоновая горутина чистит просроченные файлы каждые N секунд