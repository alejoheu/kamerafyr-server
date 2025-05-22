# Kamerafyr (Server)

This is a part of a project we have at school where we must fine speeding cars.
This server handles the data & calculation.

## Setup

I recommend you set this up using Docker:

```
docker run -d -p 8080:8080 --name "kamerafyr-server" ghcr.io/alejoheu/kamerafyr-server:latest 
```

Feel free to replace ``docker`` with ``podman`` if you use that.