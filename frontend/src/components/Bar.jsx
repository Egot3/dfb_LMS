import Nav from 'react-bootstrap/Nav'
import Navbar from 'react-bootstrap/Navbar'
import Container from 'react-bootstrap/Container'
import 'bootstrap/dist/css/bootstrap.min.css';
import "../assets/componentsStyles/bar.scss"

const Bar = () => {
    return(
        <Navbar sticky='top' className='mainNavbar'>
            <Container>
                <Navbar.Brand>LMS</Navbar.Brand>
                <Navbar.Toggle>Borgar</Navbar.Toggle>
                <Navbar.Collapse className='navItemHolder'>
                    <Nav variant='tabs' defaultActiveKey="/home">
                        <Nav.Item className='navItem'>
                            <Nav.Link>Задания</Nav.Link>
                        </Nav.Item>
                        <Nav.Item className='navItem'>
                            <Nav.Link>Оценки</Nav.Link>
                        </Nav.Item>
                        <Nav.Item className='navItem'>
                            <Nav.Link>Хз не читал</Nav.Link>
                        </Nav.Item>
                    </Nav>
                    <Nav className="justify-content-end">
                        <Nav.Item></Nav.Item>
                    </Nav>
                </Navbar.Collapse>
            </Container>
        </Navbar>
    )
}

export default Bar