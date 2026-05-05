# Etapa 1: Build
FROM golang:1.21-alpine AS builder

# Instalar dependencias necesarias
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copiar go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Compilar la aplicación
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main .

# Etapa 2: Runtime
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copiar el binario desde la etapa builder
COPY --from=builder /app/main .

# NO copies .env.example - usar variables de entorno de Render
# COPY --from=builder /app/.env.example .env  <--- ELIMINA ESTA LÍNEA

# Exponer puerto
EXPOSE 3000

# Ejecutar
CMD ["./main"]
