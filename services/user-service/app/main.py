# services/user-service/app/main.py
from fastapi import FastAPI, Depends, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.security import HTTPBearer
import uvicorn
import asyncio
import threading
from contextlib import asynccontextmanager
from sqlalchemy import text

from app.config.database import engine, Base, get_db
from app.config.settings import settings
from app.api.routes import auth, users, health ,stream
from app.middleware.logging import LoggingMiddleware
from app.grpc_server.user_service import serve_grpc

# IMPORTANT: Import all models here so they are registered with SQLAlchemy
from app.models.user import User
from app.models.auth import RefreshToken

# Global gRPC server reference
grpc_server = None
grpc_port = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global grpc_server, grpc_port

    # Startup
    print("🚀 Starting User Service...")

    # Test database connection first
    try:
        print("🔌 Testing database connection...")
        with engine.connect() as connection:
            result = connection.execute(text("SELECT 1"))
            print("✅ Database connection successful!")

            # Check if database exists and show current tables
            result = connection.execute(text("""
                                             SELECT table_name
                                             FROM information_schema.tables
                                             WHERE table_schema = 'public'
                                             """))
            existing_tables = [row[0] for row in result.fetchall()]
            print(f"📋 Existing tables: {existing_tables}")

    except Exception as e:
        print(f"❌ Database connection failed: {e}")
        print(f"📍 Database URL: {settings.DATABASE_URL}")
        # Don't exit, but show the error

    # Create tables
    try:
        print("📊 Creating database tables...")
        print(f"🔍 Models registered with Base: {list(Base.metadata.tables.keys())}")

        # Create all tables
        Base.metadata.create_all(bind=engine)
        print("✅ Database tables created successfully!")

        # Verify tables were created
        with engine.connect() as connection:
            result = connection.execute(text("""
                                             SELECT table_name
                                             FROM information_schema.tables
                                             WHERE table_schema = 'public'
                                             """))
            tables_after = [row[0] for row in result.fetchall()]
            print(f"📋 Tables after creation: {tables_after}")

    except Exception as e:
        print(f"❌ Error creating tables: {e}")
        import traceback
        print(f"🔍 Full error: {traceback.format_exc()}")

    # Start gRPC server in a separate thread
    print("🔧 Starting gRPC server...")
    try:
        grpc_server = serve_grpc()  # Let it find a free port automatically

        # Start gRPC server in background thread
        def run_grpc():
            try:
                grpc_server.wait_for_termination()
            except KeyboardInterrupt:
                print("🛑 gRPC server interrupted")
            except Exception as e:
                print(f"❌ gRPC server error: {e}")

        grpc_thread = threading.Thread(target=run_grpc, daemon=True)
        grpc_thread.start()
        print("✅ gRPC server started successfully")

    except Exception as e:
        print(f"❌ Failed to start gRPC server: {e}")
        print("⚠️  Continuing without gRPC server...")
        grpc_server = None

    yield

    # Shutdown
    print("🛑 Shutting down User Service...")
    if grpc_server:
        print("🛑 Stopping gRPC server...")
        try:
            grpc_server.stop(grace=5)
            print("✅ gRPC server stopped")
        except Exception as e:
            print(f"❌ Error stopping gRPC server: {e}")


app = FastAPI(
    title="User Service",
    description="User management service with JWT authentication and gRPC",
    version="1.0.0",
    lifespan=lifespan
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.ALLOWED_ORIGINS,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Custom middleware
app.add_middleware(LoggingMiddleware)

# Security
security = HTTPBearer()

# Routes
app.include_router(auth.router, prefix="/api/v1/auth", tags=["authentication"])
app.include_router(users.router, prefix="/api/v1/users", tags=["users"])
app.include_router(health.router, prefix="/api/v1/health", tags=["health"])
app.include_router(stream.router, prefix="/api/v1/stream", tags=["streaming"])


@app.get("/")
async def root():
    global grpc_server, grpc_port

    grpc_status = "running" if grpc_server else "not available"

    return {
        "message": "User Service API",
        "version": "1.0.0",
        "services": {
            "rest_api": "http://localhost:8000",
            "grpc": f"localhost:{grpc_port}" if grpc_port else "not available"
        },
        "status": {
            "rest_api": "running",
            "grpc": grpc_status
        }
    }


@app.get("/api/v1/status")
async def get_service_status():
    """Get detailed service status"""
    global grpc_server

    # Test database connection
    db_status = "unknown"
    try:
        with engine.connect() as connection:
            connection.execute(text("SELECT 1"))
            db_status = "connected"
    except Exception:
        db_status = "disconnected"

    return {
        "service": "user-service",
        "version": "1.0.0",
        "status": {
            "rest_api": "running",
            "grpc_server": "running" if grpc_server else "not available",
            "database": db_status
        },
        "endpoints": {
            "rest_api": "http://localhost:8000",
            "grpc": f"localhost:{grpc_port}" if grpc_port else "not available"
        }
    }


def run_servers():
    """Run both FastAPI and gRPC servers"""
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=8000,
        reload=settings.DEBUG
    )


if __name__ == "__main__":
    run_servers()