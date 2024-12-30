package main

import (
	"database/sql"
	"errors"
	"flag"
	_ "modernc.org/sqlite"
)

var dbType = flag.String("db-type", "noop", "what kind of database to use: noop")

func configureDatabase() (dbInterface, error) {
	var db dbInterface
	switch *dbType {
	case "noop":
		db = &noopDB{}
	case "sqlite":
		db = &sqliteDB{}
	default:
		return nil, errors.New("invalid database type specified")
	}
	return db, nil
}

type dbInterface interface {
	open() error
	close() error
	addGuess(puzzle string, guess string, result GuessResult)
	addHint(puzzle string, hint int)
}

type noopDB struct{}

func (db *noopDB) open() error {
	return nil
}

func (db *noopDB) close() error {
	return nil
}

func (db *noopDB) addGuess(string, string, GuessResult) {
	//NOOP
}

func (db *noopDB) addHint(string, int) {
	//NOOP
}

type sqliteDB struct {
	db *sql.DB
}

func (db *sqliteDB) openDB() error {
	dbh, err := sql.Open("sqlite", "")
	if err != nil {
		return err
	}
	db.db = dbh
	return nil
}

func (db *sqliteDB) open() error {
	//TODO implement me
	panic("implement me")
}

func (db *sqliteDB) close() error {
	//TODO implement me
	panic("implement me")
}

func (db *sqliteDB) addGuess(puzzle string, guess string, result GuessResult) {
	//TODO implement me
	panic("implement me")
}

func (db *sqliteDB) addHint(puzzle string, hint int) {
	//TODO implement me
	panic("implement me")
}
