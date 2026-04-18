package models

import "time"

type JobStatus string

const (
	StatusAssigned   JobStatus = "assigned"
	StatusAccepted   JobStatus = "accepted"
	StatusInProgress JobStatus = "in_progress"
	StatusCompleted  JobStatus = "completed"
)

type Job struct {
	ID             uint       `json:"ID" gorm:"primaryKey"`
	BookingNo      string     `json:"BookingNo"`
	ContainerID    string     `json:"ContainerID"`
	Origin         string     `json:"Origin"`
	Destination    string     `json:"Destination"`
	Status         JobStatus  `json:"Status" gorm:"type:VARCHAR(20);index"`
	AssigneeID     *uint      `json:"AssigneeID"`
	Assignee       *User      `json:"Assignee"`
	QRText         *string    `json:"QRText"`
	OCRText        *string    `json:"OCRText"`
	PhotoURL       *string    `json:"PhotoURL"`
	StartedAt      *time.Time `json:"StartedAt"`
	CompletedAt    *time.Time `json:"CompletedAt"`
	ScheduledFor   *time.Time `json:"ScheduledFor" gorm:"index"` 
	CreatedAt      time.Time  `json:"CreatedAt"`
	UpdatedAt      time.Time  `json:"UpdatedAt"`
	OriginLat      float64    `json:"originLat"`
	OriginLng      float64    `json:"originLng"`
	DestinationLat float64    `json:"destinationLat"`
	DestinationLng float64    `json:"destinationLng"`
}
