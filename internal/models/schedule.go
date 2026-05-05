package models

type ScheduleData struct {
    Identificacion string `json:"identificacion"`
    Docente        string `json:"docente"`
    Seccion        string `json:"seccion"`
    Materia        string `json:"materia"`
    Aula           string `json:"aula"`
    Turno          string `json:"turno"`
    Estado         string `json:"estado"`
    HoraEntrada    string `json:"horaEntrada"`
    HoraSalida     string `json:"horaSalida"`
}