# Portfolio Hub

Personal GitHub portfolio site based on the local software development specification.

## Stack

- Frontend: Next.js, TypeScript, Tailwind CSS, Recharts, Framer Motion
- Backend: Go, Gin, Redis
- Data: GitHub REST API, OpenAI-compatible AI API
- Deployment: Docker Compose

## Local Development

```bash
cp .env.example .env
npm install
cd apps/server && go mod download
```

Run the API:

```bash
npm run dev:server
```

Run the web app:

```bash
npm run dev:web
```

The web app defaults to `http://localhost:3000` and the API defaults to `http://localhost:8080`.
