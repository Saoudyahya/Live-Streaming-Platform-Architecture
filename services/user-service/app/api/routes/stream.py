# services/user-service/app/api/routes/stream.py
from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session
from pydantic import BaseModel
from app.config.database import get_db
from app.repository.user_repository import UserRepository
from app.models.user import User
from app.api.dependencies import get_current_user

router = APIRouter()


class ValidateStreamKeyRequest(BaseModel):
    stream_key: str
    ip_address: str


class ValidateStreamKeyResponse(BaseModel):
    valid: bool
    user_id: int = None
    username: str = None
    message: str = None


@router.post("/validate-stream-key", response_model=ValidateStreamKeyResponse)
async def validate_stream_key(
        request: ValidateStreamKeyRequest,
        db: Session = Depends(get_db)
):
    """
    Validate a stream key for RTMP authentication.
    This endpoint is called by the Stream Management Service.
    """
    try:
        # Find user by stream key
        user = db.query(User).filter(User.stream_key == request.stream_key).first()

        if not user:
            print(f"❌ Stream key not found: {request.stream_key}")
            return ValidateStreamKeyResponse(
                valid=False,
                message="Invalid stream key"
            )

        if not user.is_active:
            print(f"❌ User account inactive: {user.username}")
            return ValidateStreamKeyResponse(
                valid=False,
                message="User account is inactive"
            )

        # Log the authentication attempt
        print(f"✅ Stream key validated for user {user.username} (ID: {user.id}) from IP {request.ip_address}")

        return ValidateStreamKeyResponse(
            valid=True,
            user_id=user.id,
            username=user.username,
            message="Stream key is valid"
        )

    except Exception as e:
        print(f"❌ Error validating stream key: {e}")
        return ValidateStreamKeyResponse(
            valid=False,
            message="Internal server error"
        )


@router.get("/stream-key")
async def get_my_stream_key(
        current_user: User = Depends(get_current_user)
):
    """
    Get the current user's stream key.
    """
    return {
        "stream_key": current_user.stream_key,
        "rtmp_url": f"rtmp://localhost:1935/live/{current_user.stream_key}",
        "server_url": "rtmp://localhost:1935/live/",
        "user_id": current_user.id,
        "username": current_user.username,
        "instructions": {
            "obs_settings": {
                "server": "rtmp://localhost:1935/live/",
                "stream_key": current_user.stream_key
            },
            "ffmpeg_command": f"ffmpeg -re -i input.mp4 -c copy -f flv rtmp://localhost:1935/live/{current_user.stream_key}"
        }
    }


@router.post("/regenerate-stream-key")
async def regenerate_stream_key(
        current_user: User = Depends(get_current_user),
        db: Session = Depends(get_db)
):
    """
    Regenerate a new stream key for the current user.
    """
    from app.utils.security import generate_stream_key

    # Generate new stream key
    old_stream_key = current_user.stream_key
    new_stream_key = generate_stream_key()

    # Update user's stream key
    current_user.stream_key = new_stream_key
    db.commit()

    print(f"🔄 Stream key regenerated for user {current_user.username}: {old_stream_key} -> {new_stream_key}")

    return {
        "message": "Stream key regenerated successfully",
        "old_stream_key": old_stream_key,
        "new_stream_key": new_stream_key,
        "rtmp_url": f"rtmp://localhost:1935/live/{new_stream_key}",
        "warning": "Update your streaming software with the new stream key"
    }


@router.get("/streaming-info")
async def get_streaming_info(
        current_user: User = Depends(get_current_user)
):
    """
    Get complete streaming information for the current user.
    """
    return {
        "user": {
            "id": current_user.id,
            "username": current_user.username,
            "email": current_user.email
        },
        "streaming": {
            "stream_key": current_user.stream_key,
            "rtmp_url": f"rtmp://localhost:1935/live/{current_user.stream_key}",
            "server_url": "rtmp://localhost:1935/live/",
            "hls_url": f"http://localhost:8080/live/{current_user.stream_key}.m3u8",
            "status": "ready"
        },
        "obs_settings": {
            "service": "Custom",
            "server": "rtmp://localhost:1935/live/",
            "stream_key": current_user.stream_key
        },
        "endpoints": {
            "stream_management": "http://localhost:8080",
            "health_check": "http://localhost:8080/health",
            "stream_info": f"http://localhost:8080/rtmp/stream/{current_user.stream_key}"
        }
    }


# Public endpoint for testing (no authentication required)
@router.get("/test-validation")
async def test_stream_key_validation(
        stream_key: str,
        db: Session = Depends(get_db)
):
    """
    Test endpoint to validate a stream key (for debugging).
    """
    request = ValidateStreamKeyRequest(
        stream_key=stream_key,
        ip_address="127.0.0.1"
    )

    return await validate_stream_key(request, db)