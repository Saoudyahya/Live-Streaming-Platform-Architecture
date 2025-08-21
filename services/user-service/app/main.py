from fastapi import FastAPI, Depends, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.security import HTTPBearer
import uvicorn
from contextlib import asynccontextmanager
from sqlalchemy import text

from app.config.database import engine, Base, get_db
from app.config.settings import settings
from app.api.routes import auth, users, health
from app.middleware.logging import LoggingMiddleware

# IMPORTANT: Import all models here so they are registered with SQLAlchemy
from app.models.user import User
from app.models.auth import RefreshToken


@asynccontextmanager
async def lifespan(app: FastAPI):
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

    yield

    # Shutdown
    print("🛑 Shutting down User Service...")


app = FastAPI(
    title="User Service",
    description="User management service with JWT authentication",
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


@app.get("/")
async def root():
    return {"message": "User Service API", "version": "1.0.0"}


if __name__ == "__main__":
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=8000,
        reload=settings.DEBUG
    )