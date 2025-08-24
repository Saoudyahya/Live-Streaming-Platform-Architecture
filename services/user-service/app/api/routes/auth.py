# services/user-service/app/api/routes/auth.py - FIXED VERSION
from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session
from app.config.database import get_db
from app.schemas.auth import LoginRequest, TokenResponse, RefreshTokenRequest, ChangePasswordRequest
from app.schemas.user import UserCreate, UserResponse
from app.services.auth_service import AuthService
from app.services.user_service import UserService
from app.api.dependencies import get_current_user
from app.models.user import User
import traceback

router = APIRouter()


@router.post("/register", response_model=UserResponse, status_code=status.HTTP_201_CREATED)
async def register(user_data: UserCreate, db: Session = Depends(get_db)):
    try:
        print(f"📝 Register attempt for user: {user_data.username}, email: {user_data.email}")

        # Test database connection first
        try:
            from sqlalchemy import text
            db.execute(text("SELECT 1"))
            print("✅ Database connection test passed in register")
        except Exception as db_test_error:
            print(f"❌ Database connection test failed: {db_test_error}")
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail="Database connection failed"
            )

        # Validate input data
        if not user_data.email or not user_data.username or not user_data.password:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Email, username, and password are required"
            )

        if len(user_data.password) < 8:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Password must be at least 8 characters long"
            )

        print(f"✅ Input validation passed")

        # Create user service
        user_service = UserService(db)

        # Create user
        result = user_service.create_user(user_data)

        print(f"✅ User created successfully: {result.username} (ID: {result.id})")
        return result

    except HTTPException as he:
        print(f"❌ HTTP Exception in register: {he.detail}")
        raise he
    except Exception as e:
        print(f"💥 Unexpected error in register: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")

        # More detailed error information
        error_details = {
            "error": str(e),
            "type": type(e).__name__,
            "user_data": {
                "email": user_data.email if hasattr(user_data, 'email') else 'N/A',
                "username": user_data.username if hasattr(user_data, 'username') else 'N/A'
            }
        }

        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Registration failed: {error_details}"
        )


@router.post("/login", response_model=TokenResponse)
async def login(login_data: LoginRequest, db: Session = Depends(get_db)):
    try:
        print(f"🔐 Login attempt for email: {login_data.email}")

        # Test database connection
        from sqlalchemy import text
        db.execute(text("SELECT 1"))
        print("✅ Database connection test passed in login")

        auth_service = AuthService(db)
        result = auth_service.authenticate_user(login_data)

        print(f"✅ Login successful for: {login_data.email}")
        return result

    except HTTPException as he:
        print(f"❌ HTTP Exception in login: {he.detail}")
        raise he
    except Exception as e:
        print(f"💥 Unexpected error in login: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Login failed: {str(e)}"
        )


@router.post("/refresh", response_model=TokenResponse)
async def refresh_token(
        refresh_data: RefreshTokenRequest,
        db: Session = Depends(get_db)
):
    try:
        print(f"🔄 Token refresh attempt")

        auth_service = AuthService(db)
        result = auth_service.refresh_access_token(refresh_data.refresh_token)

        print(f"✅ Token refresh successful")
        return result

    except HTTPException as he:
        print(f"❌ HTTP Exception in refresh: {he.detail}")
        raise he
    except Exception as e:
        print(f"💥 Unexpected error in refresh: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Token refresh failed: {str(e)}"
        )


@router.post("/logout")
async def logout(
        refresh_data: RefreshTokenRequest,
        db: Session = Depends(get_db)
):
    try:
        print(f"👋 Logout attempt")

        auth_service = AuthService(db)
        success = auth_service.logout(refresh_data.refresh_token)

        message = "Logged out successfully" if success else "Already logged out"
        print(f"✅ Logout result: {message}")
        return {"message": message}

    except Exception as e:
        print(f"💥 Unexpected error in logout: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Logout failed: {str(e)}"
        )


@router.post("/change-password")
async def change_password(
        password_data: ChangePasswordRequest,
        current_user: User = Depends(get_current_user),
        db: Session = Depends(get_db)
):
    try:
        print(f"🔒 Password change attempt for user: {current_user.username}")

        user_service = UserService(db)
        success = user_service.change_password(
            current_user.id,
            password_data.current_password,
            password_data.new_password
        )

        print(f"✅ Password change successful for user: {current_user.username}")
        return {"message": "Password changed successfully"}

    except HTTPException as he:
        print(f"❌ HTTP Exception in change_password: {he.detail}")
        raise he
    except Exception as e:
        print(f"💥 Unexpected error in change_password: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Password change failed: {str(e)}"
        )


@router.get("/me", response_model=UserResponse)
async def get_current_user_info(current_user: User = Depends(get_current_user)):
    try:
        print(f"👤 Get current user info for: {current_user.username}")
        return UserResponse.model_validate(current_user)
    except Exception as e:
        print(f"💥 Unexpected error in get_current_user_info: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to get user info: {str(e)}"
        )


# Enhanced debug endpoint
@router.get("/debug/test")
async def debug_test(db: Session = Depends(get_db)):
    try:
        print("🧪 Running enhanced debug test...")

        debug_results = {
            "database": "unknown",
            "models": "unknown",
            "services": "unknown",
            "security": "unknown",
            "test_user_creation": "unknown"
        }

        # Test database
        try:
            from sqlalchemy import text
            result = db.execute(text("SELECT 1 as test"))
            row = result.fetchone()
            debug_results["database"] = "connected" if row and row[0] == 1 else "error"

            # Check table structure
            result = db.execute(text("""
                                     SELECT table_name, column_name, data_type
                                     FROM information_schema.columns
                                     WHERE table_schema = 'public'
                                       AND table_name = 'users'
                                     ORDER BY ordinal_position
                                     """))

            user_columns = []
            for row in result.fetchall():
                table_name, column_name, data_type = row
                user_columns.append(f"{column_name} ({data_type})")

            debug_results["user_table_columns"] = user_columns

        except Exception as e:
            debug_results["database"] = f"error: {str(e)}"

        # Test model imports
        try:
            from app.models.user import User
            from app.models.auth import RefreshToken
            debug_results["models"] = "imported"
        except Exception as e:
            debug_results["models"] = f"error: {str(e)}"

        # Test services
        try:
            from app.services.auth_service import AuthService
            from app.services.user_service import UserService
            debug_results["services"] = "imported"
        except Exception as e:
            debug_results["services"] = f"error: {str(e)}"

        # Test security utils
        try:
            from app.utils.security import hash_password, generate_stream_key
            test_password = hash_password("testpass123")
            test_stream_key = generate_stream_key()
            debug_results["security"] = "working"
            debug_results["test_password_hash_length"] = len(test_password)
            debug_results["test_stream_key_length"] = len(test_stream_key)
        except Exception as e:
            debug_results["security"] = f"error: {str(e)}"

        # Test user creation (without committing)
        try:
            from app.repository.user_repository import UserRepository
            from app.schemas.user import UserCreate

            user_repo = UserRepository(db)
            test_user_data = UserCreate(
                email="debug@example.com",
                username="debug_user",
                password="testpassword123"
            )

            # Test the creation process without committing
            from app.utils.security import hash_password, generate_stream_key
            from app.models.user import User

            hashed_password = hash_password(test_user_data.password)
            stream_key = generate_stream_key()

            test_user = User(
                email=test_user_data.email,
                username=test_user_data.username,
                hashed_password=hashed_password,
                stream_key=stream_key
            )

            debug_results["test_user_creation"] = "model_creation_successful"

        except Exception as e:
            debug_results["test_user_creation"] = f"error: {str(e)}"
            debug_results["test_user_creation_traceback"] = traceback.format_exc()

        print("✅ Debug test completed")
        debug_results["status"] = "success"

        return debug_results

    except Exception as e:
        print(f"💥 Debug test failed: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        return {
            "status": "error",
            "error": str(e),
            "traceback": traceback.format_exc()
        }


# Simple registration test endpoint
@router.post("/debug/test-register")
async def debug_test_register(db: Session = Depends(get_db)):
    """Test endpoint to debug registration without external input"""
    try:
        print("🧪 Testing registration process...")

        from app.schemas.user import UserCreate
        from app.services.user_service import UserService

        # Use hardcoded test data to eliminate input issues
        test_data = UserCreate(
            email="debug_reg@example.com",
            username="debug_reg_user",
            password="testpassword123",
            first_name="Debug",
            last_name="Registration"
        )

        user_service = UserService(db)
        result = user_service.create_user(test_data)

        # Clean up immediately
        from app.repository.user_repository import UserRepository
        user_repo = UserRepository(db)
        user_repo.delete_user(result.id)

        return {
            "status": "success",
            "message": "Registration test successful",
            "created_user_id": result.id,
            "created_username": result.username
        }

    except Exception as e:
        print(f"💥 Registration test failed: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        return {
            "status": "error",
            "error": str(e),
            "traceback": traceback.format_exc()
        }