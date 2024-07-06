package main

import (
    "context"
    "fmt"
    "os"
    "net/http"
    "net/url"
    "strings"
    "encoding/json"

    "github.com/shurcooL/githubv4"
    "golang.org/x/oauth2"
)

type repo struct {
    NameWithOwner string
}

type pageinfo struct {
    EndCursor   githubv4.String
    HasNextPage bool
}

type repositories struct {
    Nodes    []repo
    PageInfo pageinfo
}

type reposQuery struct {
    Viewer struct {
        Login        string
        Repositories repositories `graphql:"repositories(first: 100, after: $repoCursor)"`
    }

    Organization struct {
        Login        string
        Repositories repositories `graphql:"repositories(first: 100, after: $orgRepoCursor)"`
    } `graphql:"organization(login: $orgName )"`
}

func buildParameters() map[string]interface{} {
    return map[string]interface{}{
        "repoCursor":    (*githubv4.String)(nil),
        "orgRepoCursor": (*githubv4.String)(nil),
        "orgName":       *githubv4.NewString("github"),
    }
}

func pageForward(q reposQuery, variables map[string]interface{}) bool {
    if !q.Viewer.Repositories.PageInfo.HasNextPage && !q.Organization.Repositories.PageInfo.HasNextPage {
        return false
    }

    variables["repoCursor"] = githubv4.NewString(q.Viewer.Repositories.PageInfo.EndCursor)
    variables["orgRepoCursor"] = githubv4.NewString(q.Organization.Repositories.PageInfo.EndCursor)

    return true
}

func initializeV4Client(token string) *githubv4.Client {
    sts := oauth2.StaticTokenSource(
        &oauth2.Token{AccessToken: token},
    )
    httpClient := oauth2.NewClient(context.Background(), sts)
    return githubv4.NewClient(httpClient)
}

func printReposFromThisPage(q reposQuery) {
    for _, repo := range q.Viewer.Repositories.Nodes {
        fmt.Println(repo.NameWithOwner)
    }

    for _, repo := range q.Organization.Repositories.Nodes {
        fmt.Println(repo.NameWithOwner)
    }
}

func listAllRepos(client *githubv4.Client, extraOrg string) {
    var q reposQuery
    variables := buildParameters()
    variables["orgName"] = githubv4.String(extraOrg)

    for {
        err := client.Query(context.Background(), &q, variables)
        if err != nil {
            fmt.Println(err)
        }

        printReposFromThisPage(q)

        if !pageForward(q, variables) {
            break
        }
    }
}

// startOAuthServer starts an HTTP server to listen for the OAuth callback and capture the authorization code.
func startOAuthServer(ctx context.Context, port string, path string, codeChan chan<- string) {
    fmt.Println("TEST")
    http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
        code := r.URL.Query().Get("code")
        if code == "" {
            http.Error(w, "Code not found in the request", http.StatusBadRequest)
            return
        }
        fmt.Fprintf(w, "Authorization successful, you can close this window.")
        codeChan <- code  // Send the code back through the channel
    })

    server := &http.Server{Addr: ":" + port}

    fmt.Println("Server is starting on port:", port)
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            fmt.Println("HTTP server ListenAndServe error:", err)
        }
    }()

    // Wait for the server to be shutdown
    <-ctx.Done()
    server.Shutdown(ctx)
}

func getAccessToken(clientID, clientSecret, code string) (string, error) {

    apiURL := "https://github.com/login/oauth/access_token" // GitHub API URL, maybe something else

    requestBody := url.Values{}
    requestBody.Set("client_id", clientID)
    requestBody.Set("client_secret", clientSecret)
    requestBody.Set("code", code)
    fmt.Println(requestBody)

    req, err := http.NewRequest("POST", apiURL, strings.NewReader(requestBody.Encode()))
    if err != nil {
        return "", fmt.Errorf("request creation failed: %w", err)
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Accept", "application/json")
    fmt.Println(req)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    var tokenResponse struct {
        AccessToken string `json:"access_token"`
        TokenType   string `json:"token_type"`
        Scope       string `json:"scope"`
        Error       string `json:"error"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
        return "", fmt.Errorf("error decoding response: %w", err)
    }

    if tokenResponse.Error != "" {
        return "", fmt.Errorf("error tokenResponse from GitHub: %s", tokenResponse.Error)
    }

    return tokenResponse.AccessToken, nil
}

func main() {
    // clientID := os.Getenv("GITHUB_SEARCH_CLIENT_ID")
    // clientSecret := os.Getenv("GITHUB_SEARCH_CLIENT_SECRET")

    // fmt.Println("Starting Channel...")
    // // Channel to receive the code
    // codeChan := make(chan string)
    // ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    // defer cancel()

    // // Start the server
    // fmt.Println("Starting OAuth Server...")
    // go startOAuthServer(ctx, "8080", "/oauth/callback", codeChan)
    // fmt.Println("Server setup complete, waiting for the OAuth callback...")

    // fmt.Println("Open the following URL in your browser to authorize:")
    // fmt.Printf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=repo\n", "your_client_id_here", "http://localhost:8080/oauth/callback")

    // // Wait to receive the code or timeout
    // var code string
    // select {
    // case code := <-codeChan:
    //     fmt.Println("Received code:", code)
    //     // Use this code to get an access token or whatever you need
    // case <-ctx.Done():
    //     fmt.Println("Did not receive code in time: ", ctx.Err())
    // }

    // fmt.Println(code)

    // if clientID == "" || clientSecret == "" {
    //     fmt.Println("Please set all required environment variables: GITHUB_SEARCH_CLIENT_ID, GITHUB_SEARCH_CLIENT_SECRET.")
    //     return
    // }

    // // Get the access token
    // token, err := getAccessToken(clientID, clientSecret, code)
    // if err != nil {
    //     fmt.Println("Error getting access token:", err)
    //     return
    // }
    // if token == "" {
    //     fmt.Println("Access token is empty.")
    // }

    // Initialize GitHub v4 client with the token

    // Assuming the next argument is the organization name
    if len(os.Args) < 3 {
        fmt.Println("You have: ", os.Args)
        fmt.Println("Should be: <executable> <github_organization>")
        return
    }

    token := os.Args[1]
    client := initializeV4Client(token)

    listAllRepos(client, os.Args[2])
}
