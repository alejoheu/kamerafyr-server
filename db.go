package main

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// make the db variable public
var Database *gorm.DB

// This function initializes the database.
func InitializeDb() {
	var err error
	Database, err = gorm.Open(sqlite.Open("kamerafyr-server.db"), &gorm.Config{})
	if err != nil {
		Log.Error("could not open database", "error", err)
		return
	}

	// migrate
	Database.AutoMigrate(&LicensePlateRequest{})
}
