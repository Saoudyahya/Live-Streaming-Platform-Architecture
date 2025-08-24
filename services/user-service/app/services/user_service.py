from sqlalchemy.orm import Session
from typing import List, Optional
from app.repository.user_repository import UserRepository
from app.schemas.user import UserCreate, UserUpdate, UserResponse
from app.models.user import User
from app.utils.exceptions import UserAlreadyExistsError, UserNotFoundError, InvalidCredentialsError
from app.utils.security import verify_password
import traceback


class UserService:
    def __init__(self, db: Session):
        print("🔧 Initializing UserService...")
        self.db = db
        self.user_repo = UserRepository(db)
        print("✅ UserService initialized successfully")

    def create_user(self, user_data: UserCreate) -> UserResponse:
        try:
            print(f"👤 Creating user: {user_data.username} ({user_data.email})")

            # Check if user already exists by email
            print(f"🔍 Checking if email exists: {user_data.email}")
            existing_user = self.user_repo.get_user_by_email(user_data.email)
            if existing_user:
                print(f"❌ User with email {user_data.email} already exists")
                raise UserAlreadyExistsError()

            print(f"✅ Email {user_data.email} is available")

            # Check if username already exists
            print(f"🔍 Checking if username exists: {user_data.username}")
            existing_username = self.user_repo.get_user_by_username(user_data.username)
            if existing_username:
                print(f"❌ User with username {user_data.username} already exists")
                raise UserAlreadyExistsError()

            print(f"✅ Username {user_data.username} is available")

            # Create user
            print(f"💾 Creating user in database...")
            user = self.user_repo.create_user(user_data)

            if not user:
                raise Exception("Failed to create user - repository returned None")

            print(f"✅ User created in database: ID {user.id}")
            print(f"📊 User object attributes: {dir(user)}")

            if hasattr(user, '__dict__'):
                print(f"📊 User object dict: {user.__dict__}")

            # Convert to response model
            print(f"🔄 Converting user to UserResponse...")
            try:
                user_response = UserResponse.model_validate(user)
                print(f"✅ User converted to UserResponse successfully")
                return user_response
            except Exception as e:
                print(f"❌ Error converting user to UserResponse: {e}")
                print(f"🔍 Conversion traceback: {traceback.format_exc()}")

                # Try to create response manually for debugging
                try:
                    print(f"🔧 Attempting manual conversion...")
                    manual_response = UserResponse(
                        id=user.id,
                        email=user.email,
                        username=user.username,
                        first_name=user.first_name,
                        last_name=user.last_name,
                        bio=user.bio,
                        profile_image_url=user.profile_image_url,
                        is_active=user.is_active,
                        is_verified=user.is_verified,
                        stream_key=user.stream_key,
                        created_at=user.created_at,
                        updated_at=user.updated_at
                    )
                    print(f"✅ Manual conversion successful")
                    return manual_response
                except Exception as manual_error:
                    print(f"❌ Manual conversion also failed: {manual_error}")
                    raise e

        except (UserAlreadyExistsError) as e:
            print(f"❌ User creation failed - user already exists: {e.detail}")
            raise e
        except Exception as e:
            print(f"💥 Unexpected error in create_user: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"User creation failed: {str(e)}")

    def get_user_by_id(self, user_id: int) -> UserResponse:
        try:
            print(f"🔍 Getting user by ID: {user_id}")
            user = self.user_repo.get_user_by_id(user_id)
            if not user:
                print(f"❌ User not found: {user_id}")
                raise UserNotFoundError()

            print(f"✅ User found: {user.username}")
            return UserResponse.model_validate(user)
        except UserNotFoundError as e:
            raise e
        except Exception as e:
            print(f"💥 Unexpected error in get_user_by_id: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"Failed to get user: {str(e)}")

    def get_users(self, skip: int = 0, limit: int = 100) -> List[UserResponse]:
        try:
            print(f"📋 Getting users (skip: {skip}, limit: {limit})")
            users = self.user_repo.get_users(skip=skip, limit=limit)
            print(f"✅ Found {len(users)} users")
            return [UserResponse.model_validate(user) for user in users]
        except Exception as e:
            print(f"💥 Unexpected error in get_users: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"Failed to get users: {str(e)}")

    def update_user(self, user_id: int, user_data: UserUpdate) -> UserResponse:
        try:
            print(f"📝 Updating user: {user_id}")
            user = self.user_repo.update_user(user_id, user_data)
            if not user:
                print(f"❌ User not found for update: {user_id}")
                raise UserNotFoundError()

            print(f"✅ User updated: {user.username}")
            return UserResponse.model_validate(user)
        except UserNotFoundError as e:
            raise e
        except Exception as e:
            print(f"💥 Unexpected error in update_user: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"Failed to update user: {str(e)}")

    def delete_user(self, user_id: int) -> bool:
        try:
            print(f"🗑️ Deleting user: {user_id}")
            success = self.user_repo.delete_user(user_id)
            if not success:
                print(f"❌ User not found for deletion: {user_id}")
                raise UserNotFoundError()

            print(f"✅ User deleted: {user_id}")
            return True
        except UserNotFoundError as e:
            raise e
        except Exception as e:
            print(f"💥 Unexpected error in delete_user: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"Failed to delete user: {str(e)}")

    def change_password(self, user_id: int, current_password: str, new_password: str) -> bool:
        try:
            print(f"🔒 Changing password for user: {user_id}")

            user = self.user_repo.get_user_by_id(user_id)
            if not user:
                print(f"❌ User not found: {user_id}")
                raise UserNotFoundError()

            print(f"🔍 Verifying current password for user: {user.username}")
            if not verify_password(current_password, user.hashed_password):
                print(f"❌ Invalid current password for user: {user.username}")
                raise InvalidCredentialsError()

            print(f"✅ Current password valid, updating...")
            success = self.user_repo.update_password(user_id, new_password)

            if success:
                print(f"✅ Password updated for user: {user.username}")
            else:
                print(f"❌ Failed to update password for user: {user.username}")

            return success
        except (UserNotFoundError, InvalidCredentialsError) as e:
            raise e
        except Exception as e:
            print(f"💥 Unexpected error in change_password: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"Failed to change password: {str(e)}")