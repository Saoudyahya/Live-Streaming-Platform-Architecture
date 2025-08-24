from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session
from sqlalchemy import text
from app.config.database import get_db
import traceback

router = APIRouter()


@router.get("/")
async def health_check():
    return {"status": "healthy", "service": "user-service"}


@router.get("/db")
async def database_health(db: Session = Depends(get_db)):
    try:
        # Simple query to test database connectivity
        result = db.execute(text("SELECT 1 as test"))
        row = result.fetchone()

        if row and row[0] == 1:
            return {"status": "healthy", "database": "connected"}
        else:
            return {
                "status": "unhealthy",
                "database": "disconnected",
                "error": "Query did not return expected result"
            }
    except Exception as e:
        print(f"Database health check error: {e}")
        print(f"Error traceback: {traceback.format_exc()}")

        return {
            "status": "unhealthy",
            "database": "disconnected",
            "error": str(e)
        }