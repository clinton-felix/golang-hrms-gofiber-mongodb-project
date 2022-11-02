package main

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// creating a MongoDB struct instance
type MongoInstance struct {
	Client		*mongo.Client
	Db			*mongo.Database
}

var mg MongoInstance

const (
	dbName   = "fiber-hrms"
	mongoURI = "mongodb://localhost:27017/" + dbName
)

// creating a struct instance for the employees of the company
type Employee struct {
	ID 			string		`json:"id,omitempty" bson:"_id,omitempty"`
	Name 		string		`json:"name"`
	Salary 		float64		`json:"salary"`
	Age 		float64		`json:"age"`
}

// creating our connect function
func Connect() error {
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))
	// setting a timeout to exit blocking code after stipulated seconds
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// connecting now to the client using the right context
	err = client.Connect(ctx)
	db := client.Database(dbName)

	// handling errors
	if err != nil {
		return err
	}

	// initializing mg struct
	mg = MongoInstance{
		Client: client,
		Db: db,
	}
	return nil
}

func main() {
	// connect to the database first..
	if err:= Connect() ; err != nil {
		log.Fatal("Error: %v", err)
	}


	app := fiber.New()
	collection := mg.Db.Collection("employees")
	// using fibre handles the response and request using fibre.Ctx
	// creating the get route
	app.Get("/employee", func (c *fiber.Ctx) error {
		// opening a connection with the Mongo DB database
		query := bson.D{{}}

		// access the data of employees and capture the result in cursor
		cursor, err := collection.Find(c.Context(), query)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		// define an employee variable of type Employee and make it a slice
		var employees []Employee = make([]Employee, 0)

		// format the data received in cursor and format them to be understandable by GoLang
		if err := cursor.All(c.Context(), &employees) ; err != nil {
			c.Status(500).SendString(err.Error())
		}
		// if all goes well, return employees. No need to marshal the json file because 
		// fiber c client take care of it underhood
		return c.JSON(employees)
	})

	// creating the post Route with FIber
	app.Post("/employee", func(c *fiber.Ctx) error {
		// creating a new employee variable
		employee := new(Employee)
		// this APi reads the incoming request from user(employee details being 
		// added to the db). The Body Parser elps to also format the details into the struct template
		if err:= c.BodyParser(employee) ; err != nil{
			return c.Status(400).SendString(err.Error())
		}

		// we want mongoDB to always create its own ids.
		employee.ID = ""
		insertionResult, err := collection.InsertOne(c.Context(), employee)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		/*
			We will now use the mongo id of the just inserted result, captured in the insertion result
			to search for the corresponding data to that ID instance, and then 
			serve it to the FE. This makes us doubly sure that the data was inserted well.
			1. Query the database using bson.D key value
		*/
		filter := bson.D{{Key: "_id", Value: insertionResult.InsertedID}}	// database query
		createdRecord := collection.FindOne(c.Context(), filter)	// assign query result

		// formatting the result to the fit the Employee struct instance
		createdEmployee := new(Employee)		
		createdRecord.Decode(createdEmployee)
		
		// serve the formatted result in JSON format to the front end
		return c.Status(201).JSON(createdEmployee)
	})

	// PUT 
	app.Put("/employee/:id", func(c *fiber.Ctx) error {
		// capturing the id of the employee to be updated using c.Params
		idParam := c.Params("id")
		employeeID, err := primitive.ObjectIDFromHex(idParam)
		if err != nil {
			return c.SendStatus(400)
		}

		// get the data into the BodyParser using a variable Employee declaration
		employee := new(Employee)
		if err := c.BodyParser(employee) ; err != nil {
			return c.Status(400).SendString(err.Error())
		}

		/*
			We will build a query with Id that will find the corresponding data to the ID
			from the database, and will then replace the found data, with the new data captured
			in employee above; thus updating the database with fresh data instance
			
			1. querying the database for the employee id in question, that needs updating
			2. build an update query
		*/

		query := bson.D{{Key: "_id", Value: employeeID}}	// querying for the employee id
		// building an update query using the $set
		update := bson.D{
			{Key: "$set",
				Value: bson.D{
					{Key: "name", Value: employee.Name},
					{Key: "age", Value: employee.Age},
					{Key: "salary", Value: employee.Salary},
				},
			},
		}

		// update the database
		err = collection.FindOneAndUpdate(c.Context(), query, update).Err()
		// if there is an error, it means that the filter did not match documents
		if err != nil {
			if err == mongo.ErrNoDocuments{
				return c.SendStatus(400)		// Internal server error
			}
			return c.SendStatus(500)	// regular error
		}
		employee.ID = idParam
		return c.Status(200).JSON(employee)
	})


	app.Delete("/employee/:id", func(c *fiber.Ctx) error {
		// capturing the ID of the employer and handling errors
		employeeID, err := primitive.ObjectIDFromHex(c.Params("id"))
		if err != nil {
			return c.Status(400).SendString(err.Error())
		}
		/*
			Finding the corresp record for the ID just captured and delete
			1. query the database
			2. Use the Deleteone method of mongoInstance
			3.
		*/
		query := bson.D{{ Key: "_id", Value: employeeID}}
		result, err := collection.DeleteOne(c.Context(), &query)
		if err != nil {
			return c.SendStatus(500)		// return an internal server error
		}

		// if the data did not get deleted, then it was most likely not found. Error 404
		if result.DeletedCount < 1 {
			return c.SendStatus(404)	// not Found Error
		}
		return c.Status(200).JSON("record deleted...")
	})

	// starting our server...
	log.Fatal(app.Listen(":3000"))
}