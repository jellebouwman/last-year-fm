# Last Year FM

Figure out what year of music was important for you in a particular calendar year.

# Monorepo

## Database

The project is set up with a central database package that is the single source of truth for the application database schemas ('db:'). There are two packages, a full stack web application written in TS ('app:') and a Go worker ('worker:'), that make use of the database.
