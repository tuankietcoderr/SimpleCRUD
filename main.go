package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Todo struct {
	ID     primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Title  string             `json:"title,omitempty" bson:"title,omitempty"`
	Status int                `json:"status" bson:"status"`
}

var client *mongo.Client
var err error

func Init() {
	if err != nil {
		panic(err)
	}
	client, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		panic(err)
	}

	if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
		panic(err)
	}
	fmt.Println("Successfully connected and pinged.")
}

func Disconnet() {
	if err = client.Disconnect(context.TODO()); err != nil {
		panic(err)
	}
}

func CreateTodoEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	var toDo Todo
	json.NewDecoder(r.Body).Decode(&toDo)
	collection := client.Database("mongocruddb").Collection("todos")
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	result, err := collection.InsertOne(ctx, toDo)
	if err != nil {
		panic(err)
	}
	json.NewEncoder(w).Encode(result)
}

func GetTodosEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")

	var todos []Todo
	collection := client.Database("mongocruddb").Collection("todos")
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	result, err := collection.Find(ctx, bson.M{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}

	defer result.Close(ctx)
	for result.Next(ctx) {
		var todo Todo
		result.Decode(&todo)
		todos = append(todos, todo)
	}
	if err := result.Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	json.NewEncoder(w).Encode(todos)
}

func GetTodoEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")

	params := mux.Vars(r)
	id, _ := primitive.ObjectIDFromHex(params["id"])
	filter := bson.M{"_id": id}
	var todo Todo
	collection := client.Database("mongocruddb").Collection("todos")
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	err := collection.FindOne(ctx, filter).Decode(&todo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	json.NewEncoder(w).Encode(todo)
}

func UpdateTodoEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	params := mux.Vars(r)
	id, _ := primitive.ObjectIDFromHex(params["id"])
	collection := client.Database("mongocruddb").Collection("todos")
	var todo Todo
	json.NewDecoder(r.Body).Decode(&todo)
	filter := bson.M{"_id": id}
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	var update primitive.M
	var oldTodo Todo
	if todo.Status == 0 {
		err := collection.FindOne(ctx, filter).Decode(&oldTodo)
		if err != nil {
			panic(err)
		}
		todo.Status = oldTodo.Status
	}
	if len(todo.Title) == 0 {
		err := collection.FindOne(ctx, filter).Decode(&oldTodo)
		if err != nil {
			panic(err)
		}
		todo.Title = oldTodo.Title
	}
	update = bson.M{"status": todo.Status, "title": todo.Title}
	_, err := collection.ReplaceOne(ctx, filter, update)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	w.Write([]byte(`{"message": "Successfully updated!"}`))

}

func DeleteTodoEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	params := mux.Vars(r)
	id, _ := primitive.ObjectIDFromHex(params["id"])
	filter := bson.M{"_id": id}
	collection := client.Database("mongocruddb").Collection("todos")
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	w.Write([]byte(`{"message": "Successfully deleted!"}`))
}

var tmpl *template.Template

const html = "doc/api.html"

func InitHTML() {
	tmpl = template.Must(template.ParseFiles(html))
}

func GetAPIDocumentation(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "api.html", nil)
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Println("Production mode...")
	}
	fmt.Println("Starting application...")
	Init()
	InitHTML()
	port := os.Getenv("PORT")
	fmt.Println("Server is listening on port " + port)
	router := mux.NewRouter()
	router.HandleFunc("/", GetAPIDocumentation).Methods("GET")
	router.HandleFunc("/todo", CreateTodoEndpoint).Methods("POST")
	router.HandleFunc("/todos", GetTodosEndpoint).Methods("GET")
	router.HandleFunc("/todo/{id}", GetTodoEndpoint).Methods("GET")
	router.HandleFunc("/todo/{id}", UpdateTodoEndpoint).Methods("PUT")
	router.HandleFunc("/todo/{id}", DeleteTodoEndpoint).Methods("DELETE")
	http.ListenAndServe(":"+port, router)
	Disconnet()
}
