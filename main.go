package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"encoding/json"
	"os"

	_ "github.com/jtuchscherer/feedbacker/Godeps/_workspace/src/github.com/cznic/ql/driver"
	"github.com/jtuchscherer/feedbacker/Godeps/_workspace/src/github.com/gorilla/mux"
)

type employee struct {
	Email          string
	Name           string
	ReceivedPts    int
	UnallocatedPts int
}

var mdb *sql.DB

func main() {
	fmt.Println("About to start")
	port := os.Getenv("PORT")
	var err error
	mdb, err = sql.Open("ql", "memory://mem.db")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := mdb.Close(); err != nil {
			return
		}

		fmt.Println("OK")
	}()

	setupDatabase(mdb)

	r := mux.NewRouter()
	r.HandleFunc("/showTeamMates", showTeamMates)
	r.HandleFunc("/showUnallocatedPoints", showUnallocatedPoints)
	r.HandleFunc("/allocatePoints", allocatePoints)
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("assets/"))))

	fmt.Println("Listening at port 3000")
	err = http.ListenAndServe(fmt.Sprintf(":%s", port), r)
	if err != nil {
		panic(err)
	}
}

func allocatePoints(rw http.ResponseWriter, req *http.Request) {
	receiver := req.FormValue("receiver")
	giver := req.FormValue("giver")

	points := req.FormValue("points")
	incrementPoints(points, receiver)

	decrementPoints(points, giver)

	updatedPoints := getUpdatedPoints(receiver)
	rw.Write([]byte(fmt.Sprintf("%d", updatedPoints)))
}

func showUnallocatedPoints(rw http.ResponseWriter, req *http.Request) {
	email := req.FormValue("email")
	println(email)
	rows, err := mdb.Query(fmt.Sprintf("SELECT UnallocatedPts FROM employees where Email == \"%s\";", email))
	if err != nil {
		log.Fatal(err)
		return
	}

	cols, err := rows.Columns()
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("Columns: %v\n", cols)

	var data struct {
		UnallocatedPts int
	}

	var unallocatedPoints int
	for rows.Next() {
		if err = rows.Scan(&data.UnallocatedPts); err != nil {
			rows.Close()
			log.Fatal("That didn't work. Try again.")
			break
		}

		unallocatedPoints = data.UnallocatedPts
		//		d := fmt.Sprintf(`{"Name":"%s","ReceivedPts":%d,"UnallocatedPts":%d}`, data.Name, data.ReceivedPts, data.UnallocatedPts)
		//		fmt.Println(d)

	}
	rw.Write([]byte(fmt.Sprintf("%d", unallocatedPoints)))
}

func showTeamMates(rw http.ResponseWriter, req *http.Request) {

	rows, err := mdb.Query("SELECT * FROM employees order by ReceivedPts desc;")
	if err != nil {
		log.Fatal(err)
		return
	}

	cols, err := rows.Columns()
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("Columns: %v\n", cols)

	var data struct {
		Email          string
		Name           string
		ReceivedPts    int
		UnallocatedPts int
	}

	pivots := []employee{}
	for rows.Next() {
		if err = rows.Scan(&data.Email, &data.Name, &data.ReceivedPts, &data.UnallocatedPts); err != nil {
			rows.Close()
			log.Fatal("That didn't work. Try again.")
			break
		}

		pivots = append(pivots, data)
		//		d := fmt.Sprintf(`{"Name":"%s","ReceivedPts":%d,"UnallocatedPts":%d}`, data.Name, data.ReceivedPts, data.UnallocatedPts)
		//		fmt.Println(d)

	}

	if err = rows.Err(); err != nil {
		log.Fatal(err)
		return
	}

	d, _ := json.Marshal(pivots)
	rw.Write([]byte(d))
}

func setupDatabase(db *sql.DB) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
		return
	}

	if _, err := tx.Exec("CREATE TABLE employees (Email string, Name string, ReceivedPts int, UnallocatedPts int);"); err != nil {
		return
	}

	result, err := tx.Exec(`
	INSERT INTO employees VALUES
		($1, $2, $3, $4),
		($5, $6, $7, $8),
		($9, $10, $11, $12),
		($13, $14, $15, $16),
	;
	`,
		"snelson@pivotal.io", "Sam", 10, 2,
		"jtuchscherer@pivotal.io", "Johannes", 3, 2,
		"kcombs@pivotal.io", "Kira", 24, 10,
		"cfoundrylamb@gmail.com", "CFLamb", 25, 10,
	)
	if err != nil {
		log.Fatal(err)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Fatal(err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Fatal(err)
		return
	}

	aff, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("LastInsertId %d, RowsAffected %d\n", id, aff)
}

func incrementPoints(points string, email string) {
	tx, err := mdb.Begin()
	if err != nil {
		return
	}
	statement := fmt.Sprintf("UPDATE employees SET ReceivedPts = ReceivedPts + %s WHERE Email == \"%s\"", points, email)
	println("INCREMENT ", statement)
	result, err := tx.Exec(statement)
	if err != nil {
		log.Fatal(err)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Fatal(err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Fatal(err)
		return
	}

	aff, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("LastAllocateId %d, RowsAffected %d\n", id, aff)
}

func decrementPoints(points string, email string) {
	tx, err := mdb.Begin()
	if err != nil {
		return
	}
	statement := fmt.Sprintf("UPDATE employees SET UnallocatedPts = UnallocatedPts - %s WHERE Email == \"%s\"", points, email)
	println("DECREMENT ", statement)

	result, err := tx.Exec(statement)
	if err != nil {
		log.Fatal(err)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Fatal(err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Fatal(err)
		return
	}

	aff, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("LastAllocateId %d, RowsAffected %d\n", id, aff)

}

func getUpdatedPoints(email string) int {
	rows, err := mdb.Query(fmt.Sprintf("SELECT ReceivedPts FROM employees where Email == \"%s\";", email))
	if err != nil {
		log.Fatal(err)
	}

	cols, err := rows.Columns()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Columns: %v\n", cols)

	var data struct {
		ReceivedPts int
	}

	var receivedPts int
	for rows.Next() {
		if err = rows.Scan(&data.ReceivedPts); err != nil {
			rows.Close()
			log.Fatal("That didn't work. Try again.")
			break
		}

		receivedPts = data.ReceivedPts
		//		d := fmt.Sprintf(`{"Name":"%s","ReceivedPts":%d,"UnallocatedPts":%d}`, data.Name, data.ReceivedPts, data.UnallocatedPts)
		//		fmt.Println(d)

	}
	return receivedPts
}
