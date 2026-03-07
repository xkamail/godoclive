package main

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

func loggingMiddleware(c *fiber.Ctx) error {
	return c.Next()
}

func authMiddleware(c *fiber.Ctx) error {
	token := c.Get("Authorization")
	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	return c.Next()
}

func main() {
	app := fiber.New()
	app.Use(loggingMiddleware)

	// Public routes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	app.Get("/users", listUsers)
	app.Post("/users", createUser)
	app.Get("/users/:id", getUser)
	app.Delete("/users/:id", deleteUser)

	// API group with auth middleware
	api := app.Group("/api/v1")
	api.Use(authMiddleware)

	api.Get("/items", listItems)
	api.Get("/items/:id", getItem)
	api.Post("/items", createItem)

	app.Listen(":8080")
}

func listUsers(c *fiber.Ctx) error {
	page := c.Query("page")
	limit := c.Query("limit")
	_, _ = page, limit
	users := []User{}
	return c.JSON(users)
}

func createUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	user := User{ID: 1, Name: req.Name, Email: req.Email}
	return c.Status(http.StatusCreated).JSON(user)
}

func getUser(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	_ = id
	user := User{ID: id, Name: "Alice", Email: "alice@example.com"}
	return c.Status(http.StatusOK).JSON(user)
}

func deleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	_ = id
	return c.SendStatus(http.StatusNoContent)
}

func listItems(c *fiber.Ctx) error {
	category := c.Query("category")
	_ = category
	items := []Item{}
	return c.JSON(items)
}

func getItem(c *fiber.Ctx) error {
	id := c.Params("id")
	_ = id
	item := Item{ID: 1, Name: "Widget", Category: "tools"}
	return c.Status(http.StatusOK).JSON(item)
}

func createItem(c *fiber.Ctx) error {
	var item Item
	if err := c.BodyParser(&item); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(http.StatusCreated).JSON(item)
}
