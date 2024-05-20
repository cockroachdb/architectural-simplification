Read

``` mermaid
sequenceDiagram
    participant app
    participant cache
    participant database
    
    app->>cache: Get value
    cache-->>app: Does not exist
    app->>database: Get value
    database-->>app: Value
    app->>cache: Set value
```

Write

``` mermaid
sequenceDiagram
    participant app
    participant cache
    participant database
    
    app->>database: Set value
    app->>cache: Delete value
```