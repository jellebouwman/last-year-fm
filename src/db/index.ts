import { open } from "sqlite";
import sqlite3 from 'sqlite3'

// you would have to import / invoke this in another file
export async function openDatabase () {
  return open({
    filename: 'music.db',
    driver: sqlite3.Database
  })
}