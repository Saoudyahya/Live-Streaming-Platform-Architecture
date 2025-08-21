from sqlalchemy.orm import Session
from sqlalchemy import or_
from typing import Optional, List
from app.models.user import User
from app.schemas.user import UserCreate, UserUpdate
from app.utils.security import hash_password, generate_stream_key


class UserRepository:
    def __init__(self, db: Session):
        self.db = db

    def create_user(self, user_data: UserCreate) -> User:
        hashed_password = hash_password(user_data.password)
        stream_key = generate_stream_key()

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

        self.db.add(db_user)
        self.db.commit()
        self.db.refresh(db_user)
        return db_user

    def get_user_by_id(self, user_id: int) -> Optional[User]:
        return self.db.query(User).filter(User.id == user_id).first()

    def get_user_by_email(self, email: str) -> Optional[User]:
        return self.db.query(User).filter(User.email == email).first()

    def get_user_by_username(self, username: str) -> Optional[User]:
        return self.db.query(User).filter(User.username == username).first()

    def get_user_by_email_or_username(self, identifier: str) -> Optional[User]:
        return self.db.query(User).filter(
            or_(User.email == identifier, User.username == identifier)
        ).first()

    def get_users(self, skip: int = 0, limit: int = 100) -> List[User]:
        return self.db.query(User).offset(skip).limit(limit).all()

    def update_user(self, user_id: int, user_data: UserUpdate) -> Optional[User]:
        db_user = self.get_user_by_id(user_id)
        if not db_user:
            return None

        update_data = user_data.dict(exclude_unset=True)
        for field, value in update_data.items():
            setattr(db_user, field, value)

        self.db.commit()
        self.db.refresh(db_user)
        return db_user

    def delete_user(self, user_id: int) -> bool:
        db_user = self.get_user_by_id(user_id)
        if not db_user:
            return False

        self.db.delete(db_user)
        self.db.commit()
        return True

    def update_password(self, user_id: int, new_password: str) -> bool:
        db_user = self.get_user_by_id(user_id)
        if not db_user:
            return False

        db_user.hashed_password = hash_password(new_password)
        self.db.commit()
        return True