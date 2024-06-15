package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	mainTemplate     *template.Template
	envTemplate      *template.Template
	setupTemplate    *template.Template
	handlerTemplate  *template.Template
	routeTemplate    *template.Template
	modelTemplate    *template.Template
	responseTemplate *template.Template
)

func init() {
	mainTemplate = template.Must(template.New("main").Funcs(templateFunctions).Parse(mainTxt))
	envTemplate = template.Must(template.New("env").Funcs(templateFunctions).Parse(envTxt))
	setupTemplate = template.Must(template.New("setup").Funcs(templateFunctions).Parse(setupTxt))
	handlerTemplate = template.Must(template.New("handler").Funcs(templateFunctions).Parse(handlerTxt))
	routeTemplate = template.Must(template.New("route").Funcs(templateFunctions).Parse(routeTxt))
	modelTemplate = template.Must(template.New("model").Funcs(templateFunctions).Parse(modelTxt))
	responseTemplate = template.Must(template.New("response").Funcs(templateFunctions).Parse(responseTxt))
}

func toLower(s string) string {
	return strings.ToLower(s)
}

var templateFunctions = template.FuncMap{
	"toLower": toLower,
}

func createFolders() error {
	folders := []string{
		"outputs/app",
		"outputs/app/configs",
		"outputs/app/handler",
		"outputs/app/models",
		"outputs/app/responses",
		"outputs/app/routes",
	}

	for _, folder := range folders {
		err := os.MkdirAll(folder, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

type Field struct {
	Name string
	Type string
	Tags struct {
		JSON string // `json:"fieldName"`
		BSON string // `bson:"fieldName"`
	}
}

type EntityData struct {
	Project string
	Entity  string
	Fields  []Field
}

func createGoFile(fileName string, tmpl *template.Template, data interface{}) error {
	filePath := filepath.Join("outputs", fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return tmpl.Execute(file, data)
}

func userInput(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text()
}

func main() {
	err := createFolders()
	if err != nil {
		panic(err)
	}
	projectName := userInput("Enter the project name: ")
	entityName := userInput("Enter the entity name: ")
	fields := []Field{}
	for {
		fieldName := userInput("Enter field name (or press enter to finish): ")
		if fieldName == "" {
			break
		}
		fieldType := userInput("Enter field type: ")
		fields = append(fields, Field{
			Name: fieldName,
			Type: fieldType,
			Tags: struct {
				JSON string
				BSON string
			}{
				JSON: fmt.Sprintf("`json:\"%s\"`", strings.ToLower(fieldName)),
				BSON: fmt.Sprintf("`bson:\"%s\"`", strings.ToLower(fieldName)),
			},
		})
	}

	entityData := EntityData{
		Project: projectName,
		Entity:  entityName,
		Fields:  fields,
	}

	err = createGoFile("main.go", mainTemplate, entityData)
	if err != nil {
		panic(err)
	}

	err = createGoFile(".env", envTemplate, nil)
	if err != nil {
		panic(err)
	}

	err = createGoFile("app/configs/setup.go", setupTemplate, nil)
	if err != nil {
		panic(err)
	}

	err = createGoFile(fmt.Sprintf("app/handler/%s.go", strings.ToLower(entityName)), handlerTemplate, entityData)
	if err != nil {
		panic(err)
	}

	err = createGoFile(fmt.Sprintf("app/routes/%s.go", strings.ToLower(entityName)), routeTemplate, entityData)
	if err != nil {
		panic(err)
	}

	err = createGoFile(fmt.Sprintf("app/models/%s.go", strings.ToLower(entityName)), modelTemplate, entityData)
	if err != nil {
		panic(err)
	}

	err = createGoFile(fmt.Sprintf("app/responses/%s.go", strings.ToLower(entityName)), responseTemplate, entityData)
	if err != nil {
		panic(err)
	}
}

// Templates (define as constants or load from files)
const mainTxt = `package main

import (
	"{{.Project}}/app/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"log"
)

func main() {
	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		log.Printf("Incoming request: %s %s", c.Method(), c.OriginalURL())
		return c.Next()
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:4200",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		AllowHeaders:     "Content-Type, Authorization",
		AllowCredentials: true,
	}))

	routes.Setup{{.Entity}}Routes(app)

	log.Fatal(app.Listen(":8080"))
}`

const envTxt = `MONGO_URI=mongodb+srv://localhost:1234@cluster0.r0x0o4h.mongodb.net/?retryWrites=true&w=majority&appName=Cluster0`

const setupTxt = `package configs

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

func ConnectDB() *mongo.Client {
	client, err := mongo.NewClient(options.Client().ApplyURI(EnvMongoURI()))
	if err != nil {
		log.Fatal(err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB")
	return client
}

var DB *mongo.Client = ConnectDB()

func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	collection := client.Database("Test_DB").Collection(collectionName)
	return collection
}`
const handlerTxt = `package handler

import (
    "{{.Project}}/app/configs"
    "{{.Project}}/app/models"
    "{{.Project}}/app/responses"
    "context"
    "github.com/go-playground/validator/v10"
    "github.com/gofiber/fiber/v2"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "go.mongodb.org/mongo-driver/mongo"
    "net/http"
    "time"
)

var {{.Entity}}Collection *mongo.Collection = configs.GetCollection(configs.DB, "{{.Entity | toLower}}s")

var validate = validator.New()

func Create{{.Entity}}(c *fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    var {{.Entity | toLower}} models.{{.Entity}}
    defer cancel()

    if err := c.BodyParser(&{{.Entity | toLower}}); err != nil {
        return c.Status(http.StatusBadRequest).JSON(responses.{{.Entity}}Response{Status: http.StatusBadRequest, Message: "error", Data: &fiber.Map{"data": err.Error()}})
    }

    if validationErr := validate.Struct(&{{.Entity | toLower}}); validationErr != nil {
        return c.Status(http.StatusBadRequest).JSON(responses.{{.Entity}}Response{Status: http.StatusBadRequest, Message: "error", Data: &fiber.Map{"data": validationErr.Error()}})
    }

    new{{.Entity}} := models.{{.Entity}}{
        ID: primitive.NewObjectID(),
        {{range .Fields}}{{.Name}}: {{$.Entity | toLower}}.{{.Name}},
        {{end}}
    }

    result, err := {{.Entity}}Collection.InsertOne(ctx, new{{.Entity}})
    if err != nil {
        return c.Status(http.StatusInternalServerError).JSON(responses.{{.Entity}}Response{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}})
    }

    return c.Status(http.StatusCreated).JSON(responses.{{.Entity}}Response{Status: http.StatusCreated, Message: "success", Data: &fiber.Map{"data": result}})
}

func Get{{.Entity}}s(c *fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    var {{.Entity | toLower}}s []models.{{.Entity}}
    defer cancel()

    results, err := {{.Entity}}Collection.Find(ctx, bson.M{})
    if err != nil {
        return c.Status(http.StatusInternalServerError).JSON(responses.{{.Entity}}Response{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}})
    }

    defer results.Close(ctx)
    for results.Next(ctx) {
        var single{{.Entity}} models.{{.Entity}}
        if err = results.Decode(&single{{.Entity}}); err != nil {
            return c.Status(http.StatusInternalServerError).JSON(responses.{{.Entity}}Response{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}})
        }
        {{.Entity | toLower}}s = append({{.Entity | toLower}}s, single{{.Entity}})
    }

    return c.Status(http.StatusOK).JSON(
        responses.{{.Entity}}Response{Status: http.StatusOK, Message: "success", Data: &fiber.Map{"data": {{.Entity | toLower}}s}},
    )
}

func Get{{.Entity}}(c *fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    {{.Entity | toLower}}Id := c.Params("{{.Entity | toLower}}Id")
    var {{.Entity | toLower}} models.{{.Entity}}
    defer cancel()

    objId, _ := primitive.ObjectIDFromHex({{.Entity | toLower}}Id)

    err := {{.Entity}}Collection.FindOne(ctx, bson.M{"_id": objId}).Decode(&{{.Entity | toLower}})
    if err != nil {
        return c.Status(http.StatusInternalServerError).JSON(
            responses.{{.Entity}}Response{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}},
        )
    }

    return c.Status(http.StatusOK).JSON(
        responses.{{.Entity}}Response{Status: http.StatusOK, Message: "success", Data: &fiber.Map{"data": {{.Entity | toLower}}}},
    )
}

func Edit{{.Entity}}(c *fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    {{.Entity | toLower}}Id := c.Params("{{.Entity | toLower}}Id")
    var {{.Entity | toLower}} models.{{.Entity}}
    defer cancel()

    objId, _ := primitive.ObjectIDFromHex({{.Entity | toLower}}Id)

    if err := c.BodyParser(&{{.Entity | toLower}}); err != nil {
        return c.Status(http.StatusBadRequest).JSON(
            responses.{{.Entity}}Response{Status: http.StatusBadRequest, Message: "error", Data: &fiber.Map{"data": err.Error()}},
        )
    }

    if validationErr := validate.Struct(&{{.Entity | toLower}}); validationErr != nil {
        return c.Status(http.StatusBadRequest).JSON(
            responses.{{.Entity}}Response{Status: http.StatusBadRequest, Message: "error", Data: &fiber.Map{"data": validationErr.Error()}},
        )
    }

    update := bson.M{
        "$set": bson.M{
            {{range .Fields}}
            "{{.Name | toLower}}": {{$.Entity | toLower}}.{{.Name}},
            {{end}}
        },
    }

    result, err := {{.Entity}}Collection.UpdateOne(ctx, bson.M{"_id": objId}, update)
    if err != nil {
        return c.Status(http.StatusInternalServerError).JSON(
            responses.{{.Entity}}Response{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}},
        )
    }

    return c.Status(http.StatusOK).JSON(
        responses.{{.Entity}}Response{Status: http.StatusOK, Message: "success", Data: &fiber.Map{"data": result}},
    )
}

func Delete{{.Entity}}(c *fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    {{.Entity | toLower}}Id := c.Params("{{.Entity | toLower}}Id")
    defer cancel()

    objId, _ := primitive.ObjectIDFromHex({{.Entity | toLower}}Id)

    result, err := {{.Entity}}Collection.DeleteOne(ctx, bson.M{"_id": objId})
    if err != nil {
        return c.Status(http.StatusInternalServerError).JSON(
            responses.{{.Entity}}Response{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}},
        )
    }

    if result.DeletedCount < 1 {
        return c.Status(http.StatusNotFound).JSON(
            responses.{{.Entity}}Response{Status: http.StatusNotFound, Message: "error", Data: &fiber.Map{"data": "Document not found"}},
        )
    }

    return c.Status(http.StatusOK).JSON(
        responses.{{.Entity}}Response{Status: http.StatusOK, Message: "success", Data: &fiber.Map{"data": "Document successfully deleted"}},
    )
}`

const routeTxt = `package routes

import (
	"{{.Project}}/app/handler"
	"github.com/gofiber/fiber/v2"
)

func Setup{{.Entity}}Routes(app *fiber.App) {
	api := app.Group("/api")
	{{.Entity}} := api.Group("/{{.Entity | toLower}}s")
	{{.Entity}}.Post("/", handler.Create{{.Entity}})
	{{.Entity}}.Get("/", handler.Get{{.Entity}}s)
	{{.Entity}}.Get("/:{{.Entity | toLower}}Id", handler.Get{{.Entity}})
	{{.Entity}}.Put("/:{{.Entity | toLower}}Id", handler.Edit{{.Entity}})
	{{.Entity}}.Delete("/:{{.Entity | toLower}}Id", handler.Delete{{.Entity}})
}`

const modelTxt = `package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type {{.Entity}} struct {
	ID primitive.ObjectID ` + "`bson:\"_id\" json:\"id\"`" + `
	{{range .Fields}}
	{{.Name}} {{.Type}} ` + "`bson:\"{{.Name | toLower}}\" json:\"{{.Name | toLower}}\"`" + `
	{{end}}
}
`

const responseTxt = `package responses

import "github.com/gofiber/fiber/v2"

type {{.Entity}}Response struct {
	Status  int       ` + "`json:\"status\"`" + `
	Message string    ` + "`json:\"message\"`" + `
	Data    *fiber.Map ` + "`json:\"data\"`" + `
}`
