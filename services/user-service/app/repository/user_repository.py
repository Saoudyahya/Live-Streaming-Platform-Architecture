from sqlalchemy.orm import Session
from sqlalchemy import or_
from typing import Optional, List
from app.models.user import User
from app.schemas.user import UserCreate, UserUpdate
from app.utils.security import hash_password, generate_stream_key
import traceback


class UserRepository:
    def __init__(self, db: Session):
        print("🔧 Initializing UserRepository...")
        self.db = db
        print("✅ UserRepository initialized successfully")

    def create_user(self, user_data: UserCreate) -> User:
        try:
            print(f"👤 Creating user in repository: {user_data.username}")

            # Hash password
            print(f"🔐 Hashing password...")
            hashed_password = hash_password(user_data.password)
            print(f"✅ Password hashed (length: {len(hashed_password)})")

            # Generate stream key
            print(f"🔑 Generating stream key...")
            stream_key = generate_stream_key()
            print(f"✅ Stream key generated (length: {len(stream_key)})")

            # Create user object
            print(f"🏗️ Creating User object...")
            db_user = User(
                email=user_data.email,
                username=user_data.username,
                hashed_password=hashed_password,
                first_name=user_data.first_name,
                last_name=user_data.last_name,
                bio=user_data.bio,
                profile_image_url=user_data.profile_image_url,
                stream_key=stream_key
            )

            print(f"✅ User object created")
            print(f"📊 User object before save: {db_user.__dict__}")

            # Add to database
            print(f"💾 Adding user to database session...")
            self.db.add(db_user)

            print(f"💾 Committing database transaction...")
            self.db.commit()

            print(f"🔄 Refreshing user object...")
            self.db.refresh(db_user)

            print(f"✅ User created successfully in database")
            print(f"📊 User object after save: {db_user.__dict__}")

            # Verify the user was actually saved
            if db_user.id:
                print(f"✅ User has ID: {db_user.id}")
            else:
                print(f"❌ User ID is None after commit")

            return db_user

        except Exception as e:
            print(f"💥 Error creating user in repository: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")

            # Rollback the transaction
            try:
                print(f"🔄 Rolling back transaction...")
                self.db.rollback()
            except Exception as rollback_error:
                print(f"❌ Error during rollback: {rollback_error}")

            raise Exception(f"Failed to create user in database: {str(e)}")

    def get_user_by_id(self, user_id: int) -> Optional[User]:
        try:
            print(f"🔍 Looking up user by ID: {user_id}")
            user = self.db.query(User).filter(User.id == user_id).first()

            if user:
                print(f"✅ User found: {user.username}")
            else:
                print(f"❌ User not found with ID: {user_id}")

            return user
        except Exception as e:
            print(f"💥 Error getting user by ID: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            return None

    def get_user_by_email(self, email: str) -> Optional[User]:
        try:
            print(f"🔍 Looking up user by email: {email}")
            user = self.db.query(User).filter(User.email == email).first()

            if user:
                print(f"✅ User found by email: {user.username}")
            else:
                print(f"ℹ️ No user found with email: {email}")

            return user
        except Exception as e:
            print(f"💥 Error getting user by email: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            return None

    def get_user_by_username(self, username: str) -> Optional[User]:
        try:
            print(f"🔍 Looking up user by username: {username}")
            user = self.db.query(User).filter(User.username == username).first()

            if user:
                print(f"✅ User found by username: {user.username}")
            else:
                print(f"ℹ️ No user found with username: {username}")

            return user
        except Exception as e:
            print(f"💥 Error getting user by username: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            return None

    def get_user_by_email_or_username(self, identifier: str) -> Optional[User]:
        try:
            print(f"🔍 Looking up user by email or username: {identifier}")
            user = self.db.query(User).filter(
                or_(User.email == identifier, User.username == identifier)
            ).first()

            if user:
                print(f"✅ User found: {user.username}")
            else:
                print(f"ℹ️ No user found with identifier: {identifier}")

            return user
        except Exception as e:
            print(f"💥 Error getting user by email or username: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            return None

    def get_users(self, skip: int = 0, limit: int = 100) -> List[User]:
        try:
            print(f"📋 Getting users (skip: {skip}, limit: {limit})")
            users = self.db.query(User).offset(skip).limit(limit).all()
            print(f"✅ Found {len(users)} users")
            return users
        except Exception as e:
            print(f"💥 Error getting users: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            return []

    def update_user(self, user_id: int, user_data: UserUpdate) -> Optional[User]:
        try:
            print(f"📝 Updating user: {user_id}")

            db_user = self.get_user_by_id(user_id)
            if not db_user:
                print(f"❌ User not found for update: {user_id}")
                return None

            print(f"🔄 Updating user fields...")
            update_data = user_data.dict(exclude_unset=True)
            for field, value in update_data.items():
                print(f"📝 Setting {field} = {value}")
                setattr(db_user, field, value)

            print(f"💾 Committing user update...")
            self.db.commit()
            self.db.refresh(db_user)

            print(f"✅ User updated successfully: {db_user.username}")
            return db_user
        except Exception as e:
            print(f"💥 Error updating user: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            try:
                self.db.rollback()
            except:
                pass
            return None

    def delete_user(self, user_id: int) -> bool:
        try:
            print(f"🗑️ Deleting user: {user_id}")

            db_user = self.get_user_by_id(user_id)
            if not db_user:
                print(f"❌ User not found for deletion: {user_id}")
                return False

            print(f"💾 Deleting user from database...")
            self.db.delete(db_user)
            self.db.commit()

            print(f"✅ User deleted successfully: {user_id}")
            return True
        except Exception as e:
            print(f"💥 Error deleting user: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            try:
                self.db.rollback()
            except:
                pass
            return False

    def update_password(self, user_id: int, new_password: str) -> bool:
        try:
            print(f"🔒 Updating password for user: {user_id}")

            db_user = self.get_user_by_id(user_id)
            if not db_user:
                print(f"❌ User not found for password update: {user_id}")
                return False

            print(f"🔐 Hashing new password...")
            db_user.hashed_password = hash_password(new_password)

            print(f"💾 Committing password update...")
            self.db.commit()

            print(f"✅ Password updated successfully for user: {user_id}")
            return True
        except Exception as e:
            print(f"💥 Error updating password: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            try:
                self.db.rollback()
            except:
                pass
            return False