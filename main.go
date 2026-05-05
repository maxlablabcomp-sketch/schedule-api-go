package main

import (
    "log"
    "net/http"
    "os"
    "time"

    "github.com/gorilla/mux"
    "github.com/joho/godotenv"

    "schedule-api/internal/cache"
    "schedule-api/internal/controller"
    "schedule-api/internal/service"
)

func main() {
    // Cargar variables de entorno
    if err := godotenv.Load(); err != nil {
        log.Println("⚠️ No .env file found, using environment variables")
    }

    // Inicializar Redis
    redisCache, err := cache.NewRedisCache()
    if err != nil {
        log.Printf("⚠️ Redis not available: %v, using fallback only", err)
        redisCache = nil
    }

    // Inicializar servicio
    scheduleService := service.NewScheduleService(redisCache)
    
    // Iniciar actualizaciones periódicas en segundo plano
    go scheduleService.Start()

    // Inicializar controlador
    scheduleController := controller.NewScheduleController(scheduleService)

    // Configurar rutas
    router := mux.NewRouter()
    
    // Health check endpoint
    router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ok","time":"` + time.Now().Format(time.RFC3339) + `"}`))
    }).Methods("GET")
    
    router.HandleFunc("/schedule/refresh", scheduleController.Refresh).Methods("GET")
    router.HandleFunc("/schedule/F", scheduleController.GetScheduleF).Methods("GET")
    router.HandleFunc("/schedule/G", scheduleController.GetScheduleG).Methods("GET")

    // Configurar puerto
    port := os.Getenv("PORT")
    if port == "" {
        port = "3000"
    }

    // Configurar servidor
    server := &http.Server{
        Addr:         ":" + port,
        Handler:      router,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    log.Printf("🚀 Application is running on http://localhost:%s", port)
    log.Printf("📋 Endpoints:")
    log.Printf("   GET /health - Health check")
    log.Printf("   GET /schedule/refresh - Refresh cache")
    log.Printf("   GET /schedule/F - Get schedule for Bloque F")
    log.Printf("   GET /schedule/G - Get schedule for Bloque G")
    
    if err := server.ListenAndServe(); err != nil {
        log.Fatal("❌ Failed to start server:", err)
    }
}