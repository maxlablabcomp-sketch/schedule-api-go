package service

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "sync"
    "time"

    "schedule-api/internal/cache"
    "schedule-api/internal/models"
)

type ScheduleService struct {
    redis       *cache.RedisCache
    fallback    map[string][]models.ScheduleData
    mu          sync.RWMutex
    apiURL      string
    apiTimeout  time.Duration
    isUpdating  bool
    updateMutex sync.Mutex
}

func NewScheduleService(redisCache *cache.RedisCache) *ScheduleService {
    apiURL := os.Getenv("API_BASE_URL")
    if apiURL == "" {
        apiURL = "https://cloud.urbe.edu/web/v1/core/labComp/rotafolio"
    }

    apiTimeout := 300 * time.Second

    return &ScheduleService{
        redis:      redisCache,
        fallback:   make(map[string][]models.ScheduleData),
        apiURL:     apiURL,
        apiTimeout: apiTimeout,
    }
}

func (s *ScheduleService) Start() {
    log.Println("🚀 ScheduleService iniciado")
    log.Printf("⏰ Timeout configurado: %v", s.apiTimeout)
    log.Printf("🌐 API Base URL: %s", s.apiURL)

    // Primera actualización
    log.Println("📡 Ejecutando primera actualización...")
    s.update()

    // Actualizaciones periódicas
    intervalStr := os.Getenv("UPDATE_INTERVAL")
    if intervalStr == "" {
        intervalStr = "60000"
    }
    interval, err := time.ParseDuration(intervalStr + "ms")
    if err != nil {
        interval = 60 * time.Second
    }
    log.Printf("⏰ Actualizaciones programadas cada %v", interval)

    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            s.update()
        }
    }()
}

func (s *ScheduleService) update() {
    // Evitar actualizaciones concurrentes
    s.updateMutex.Lock()
    if s.isUpdating {
        s.updateMutex.Unlock()
        log.Println("⏭️ Actualización ya en progreso, saltando...")
        return
    }
    s.isUpdating = true
    s.updateMutex.Unlock()

    defer func() {
        s.updateMutex.Lock()
        s.isUpdating = false
        s.updateMutex.Unlock()
    }()

    log.Println("=================================")
    log.Printf("🕒 %s - Iniciando actualización", time.Now().Format("2006-01-02 15:04:05"))

    client := &http.Client{
        Timeout: s.apiTimeout,
    }

    // Fetch en paralelo
    type result struct {
        key  string
        data []models.ScheduleData
        err  error
    }

    results := make(chan result, 2)

    go func() {
        log.Println("📡 Solicitando BLOQUE F (idBloque=6)...")
        startF := time.Now()
        data, err := s.fetchSchedule(client, "6")
        if err != nil {
            log.Printf("❌ Error fetching Bloque F: %v", err)
            results <- result{key: "F", data: nil, err: err}
            return
        }
        log.Printf("✅ Bloque F recibido en %v", time.Since(startF))
        results <- result{key: "F", data: data, err: nil}
    }()

    go func() {
        log.Println("📡 Solicitando BLOQUE G (idBloque=7)...")
        startG := time.Now()
        data, err := s.fetchSchedule(client, "7")
        if err != nil {
            log.Printf("❌ Error fetching Bloque G: %v", err)
            results <- result{key: "G", data: nil, err: err}
            return
        }
        log.Printf("✅ Bloque G recibido en %v", time.Since(startG))
        results <- result{key: "G", data: data, err: nil}
    }()

    ttlStr := os.Getenv("CACHE_TTL")
    if ttlStr == "" {
        ttlStr = "60000"
    }
    ttl, _ := strconv.Atoi(ttlStr)

    // Procesar resultados
    fData := make([]models.ScheduleData, 0)
    gData := make([]models.ScheduleData, 0)

    for i := 0; i < 2; i++ {
        res := <-results
        if res.err == nil && res.data != nil {
            if res.key == "F" {
                fData = res.data
            } else if res.key == "G" {
                gData = res.data
            }
        }
    }

    // Guardar en Redis
    if s.redis != nil {
        if err := s.redis.Ping(); err == nil {
            if len(fData) > 0 {
                if err := s.redis.Set("F", fData, ttl); err != nil {
                    log.Printf("❌ Error saving Bloque F to Redis: %v", err)
                }
            }
            if len(gData) > 0 {
                if err := s.redis.Set("G", gData, ttl); err != nil {
                    log.Printf("❌ Error saving Bloque G to Redis: %v", err)
                }
            }
        } else {
            log.Printf("⚠️ Redis no disponible: %v", err)
        }
    }

    // Guardar en fallback local
    s.mu.Lock()
    if len(fData) > 0 {
        s.fallback["F"] = fData
    }
    if len(gData) > 0 {
        s.fallback["G"] = gData
    }
    s.mu.Unlock()

    log.Printf("✅ Datos guardados - F: %d items, G: %d items", len(fData), len(gData))
    log.Println("=================================")
}

func (s *ScheduleService) fetchSchedule(client *http.Client, idBloque string) ([]models.ScheduleData, error) {
    url := fmt.Sprintf("%s?idBloque=%s", s.apiURL, idBloque)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
    req.Header.Set("Accept", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
    }

    var data []models.ScheduleData
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }

    return data, nil
}

func (s *ScheduleService) GetScheduleF() ([]models.ScheduleData, error) {
    log.Printf("🔍 GET /schedule/F - %s", time.Now().Format("2006-01-02 15:04:05"))

    // Intentar obtener de Redis
    if s.redis != nil {
        if err := s.redis.Ping(); err == nil {
            var data []models.ScheduleData
            if err := s.redis.Get("F", &data); err == nil && len(data) > 0 {
                log.Printf("✅ Datos obtenidos de Redis: %d items", len(data))
                return data, nil
            }
        }
    }

    // Usar fallback
    s.mu.RLock()
    defer s.mu.RUnlock()

    fallback, exists := s.fallback["F"]
    if !exists || len(fallback) == 0 {
        log.Printf("⚠️ No hay datos disponibles en fallback")
        return []models.ScheduleData{}, nil
    }

    log.Printf("⚠️ Usando fallback local: %d items", len(fallback))
    return fallback, nil
}

func (s *ScheduleService) GetScheduleG() ([]models.ScheduleData, error) {
    log.Printf("🔍 GET /schedule/G - %s", time.Now().Format("2006-01-02 15:04:05"))

    // Intentar obtener de Redis
    if s.redis != nil {
        if err := s.redis.Ping(); err == nil {
            var data []models.ScheduleData
            if err := s.redis.Get("G", &data); err == nil && len(data) > 0 {
                log.Printf("✅ Datos obtenidos de Redis: %d items", len(data))
                return data, nil
            }
        }
    }

    // Usar fallback
    s.mu.RLock()
    defer s.mu.RUnlock()

    fallback, exists := s.fallback["G"]
    if !exists || len(fallback) == 0 {
        log.Printf("⚠️ No hay datos disponibles en fallback")
        return []models.ScheduleData{}, nil
    }

    log.Printf("⚠️ Usando fallback local: %d items", len(fallback))
    return fallback, nil
}