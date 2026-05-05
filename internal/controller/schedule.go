package controller

import (
    "encoding/json"
    "net/http"

    "schedule-api/internal/service"
)

type ScheduleController struct {
    service *service.ScheduleService
}

func NewScheduleController(service *service.ScheduleService) *ScheduleController {
    return &ScheduleController{
        service: service,
    }
}

type RefreshResponse struct {
    Status int `json:"status"`
}

type ScheduleResponse struct {
    Bloque   string      `json:"Bloque"`
    Schedule interface{} `json:"schedule"`
}

func (c *ScheduleController) Refresh(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(RefreshResponse{Status: 200})
}

func (c *ScheduleController) GetScheduleF(w http.ResponseWriter, r *http.Request) {
    schedule, err := c.service.GetScheduleF()
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }

    w.Header().Set("Content-Type", "application/json")
    response := ScheduleResponse{
        Bloque:   "F",
        Schedule: schedule,
    }
    json.NewEncoder(w).Encode(response)
}

func (c *ScheduleController) GetScheduleG(w http.ResponseWriter, r *http.Request) {
    schedule, err := c.service.GetScheduleG()
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }

    w.Header().Set("Content-Type", "application/json")
    response := ScheduleResponse{
        Bloque:   "G",
        Schedule: schedule,
    }
    json.NewEncoder(w).Encode(response)
}