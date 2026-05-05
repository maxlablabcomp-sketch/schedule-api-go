package service

import (
    "log"
    "os"
    "strconv"
    "sync"
    "time"

    "github.com/go-resty/resty/v2"
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
    client      *resty.Client
}

func NewScheduleService(redisCache *cache.RedisCache) *ScheduleService {
    apiURL := os.Getenv("API_BASE_URL")
    if apiURL == "" {
        apiURL = "https://cloud.urbe.edu/web/v1/core/labComp/rotafolio"
    }

    apiTimeout := 300 * time.Second

    client := resty.New().
        SetTimeout(apiTimeout).
        SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
        SetHeader("Accept", "application/json, text/plain, */*").
        SetHeader("Accept-Language", "es-ES,es;q=0.9").
        SetHeader("Accept-Encoding", "gzip, deflate, br").
        SetHeader("Connection", "keep-alive").
        SetHeader("Cache-Control", "no-cache").
        SetHeader("Pragma", "no-cache").
        SetHeader("Sec-Fetch-Dest", "empty").
        SetHeader("Sec-Fetch-Mode", "cors").
        SetHeader("Sec-Fetch-Site", "same-origin").
        SetHeader("Referer", "https://cloud.urbe.edu/").
        SetHeader("Origin", "https://cloud.urbe.edu")

    return &ScheduleService{
        redis:      redisCache,
        fallback:   make(map[string][]models.ScheduleData),
        apiURL:     apiURL,
        apiTimeout: apiTimeout,
        client:     client,
    }
}

func (s *ScheduleService) Start() {
    log.Println("🚀 ScheduleService iniciado")
    log.Printf("⏰ Timeout configurado: %v", s.apiTimeout)
    log.Printf("🌐 API Base URL: %s", s.apiURL)

    log.Println("📡 Ejecutando primera actualización...")
    s.update()

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

    type result struct {
        key  string
        data []models.ScheduleData
        err  error
    }

    results := make(chan result, 2)

    go func() {
        log.Println("📡 Solicitando BLOQUE F (idBloque=6)...")
        startF := time.Now()

        var fData []models.ScheduleData

        resp, err := s.client.R().
            SetQueryParam("idBloque", "6").
            SetResult(&fData).
            Get(s.apiURL)

        if err != nil {
            log.Printf("❌ Error fetching Bloque F: %v", err)
            results <- result{key: "F", data: nil, err: err}
            return
        }

        if resp.StatusCode() != 200 {
            log.Printf("❌ Error fetching Bloque F: status %d", resp.StatusCode())
            results <- result{key: "F", data: nil, err: err}
            return
        }

        log.Printf("✅ Bloque F recibido en %v", time.Since(startF))
        log.Printf("📦 Items F: %d", len(fData))
        results <- result{key: "F", data: fData, err: nil}
    }()

    go func() {
        log.Println("📡 Solicitando BLOQUE G (idBloque=7)...")
        startG := time.Now()

        var gData []models.ScheduleData

        resp, err := s.client.R().
            SetQueryParam("idBloque", "7").
            SetResult(&gData).
            Get(s.apiURL)

        if err != nil {
            log.Printf("❌ Error fetching Bloque G: %v", err)
            results <- result{key: "G", data: nil, err: err}
            return
        }

        if resp.StatusCode() != 200 {
            log.Printf("❌ Error fetching Bloque G: status %d", resp.StatusCode())
            results <- result{key: "G", data: nil, err: err}
            return
        }

        log.Printf("✅ Bloque G recibido en %v", time.Since(startG))
        log.Printf("📦 Items G: %d", len(gData))
        results <- result{key: "G", data: gData, err: nil}
    }()

    ttlStr := os.Getenv("CACHE_TTL")
    if ttlStr == "" {
        ttlStr = "60000"
    }
    ttl, _ := strconv.Atoi(ttlStr)

    fData := make([]models.ScheduleData, 0)
    gData := make([]models.ScheduleData, 0)

    for i := 0; i < 2; i++ {
        res := <-results
        if res.err == nil && res.data != nil && len(res.data) > 0 {
            if res.key == "F" {
                fData = res.data
            } else if res.key == "G" {
                gData = res.data
            }
        }
    }

    if len(fData) > 0 || len(gData) > 0 {
        if s.redis != nil {
            if err := s.redis.Ping(); err == nil {
                if len(fData) > 0 {
                    s.redis.Set("F", fData, ttl)
                }
                if len(gData) > 0 {
                    s.redis.Set("G", gData, ttl)
                }
            }
        }

        s.mu.Lock()
        if len(fData) > 0 {
            s.fallback["F"] = fData
        }
        if len(gData) > 0 {
            s.fallback["G"] = gData
        }
        s.mu.Unlock()
    }

    log.Printf("✅ Datos guardados - F: %d items, G: %d items", len(fData), len(gData))
    log.Println("=================================")
}

func (s *ScheduleService) GetScheduleF() ([]models.ScheduleData, error) {
    log.Printf("🔍 GET /schedule/F - %s", time.Now().Format("2006-01-02 15:04:05"))

    if s.redis != nil {
        if err := s.redis.Ping(); err == nil {
            var data []models.ScheduleData
            if err := s.redis.Get("F", &data); err == nil && len(data) > 0 {
                log.Printf("✅ Datos obtenidos de Redis: %d items", len(data))
                return data, nil
            }
        }
    }

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

    if s.redis != nil {
        if err := s.redis.Ping(); err == nil {
            var data []models.ScheduleData
            if err := s.redis.Get("G", &data); err == nil && len(data) > 0 {
                log.Printf("✅ Datos obtenidos de Redis: %d items", len(data))
                return data, nil
            }
        }
    }

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